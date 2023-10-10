package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/contrun/go-p2p-pipes/pb"
	"github.com/contrun/go-p2p-pipes/pkg/daemon"
	multierror "github.com/hashicorp/go-multierror"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	ps "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var log = logging.Logger("server")

type Server struct {
	pb.UnimplementedP2PPipeServer
	ctx           context.Context
	done          chan struct{}
	ListenNetwork string
	ListenAddr    string
	Listener      net.Listener
	Daemon        *daemon.Daemon
}

func NewServer(ctx context.Context, c Config, done chan struct{}, extra_opts ...libp2p.Option) (*Server, error) {
	var s Server
	s.done = done
	daemon, err := newDaemonFromConfig(ctx, c, extra_opts...)
	if err != nil {
		return nil, err
	}

	s.Daemon = daemon
	s.ctx = ctx
	s.ListenNetwork, s.ListenAddr = parse_address(c.ListenAddr)
	log.Infow("Listening grpc on", "network", s.ListenNetwork, "address", s.ListenAddr)
	l, err := net.Listen(s.ListenNetwork, s.ListenAddr)
	if err != nil {
		daemon.Close()
		return nil, err
	}
	s.Listener = l

	go s.listen()
	go s.trapSignals()

	return &s, nil
}

func (server *Server) trapSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case s := <-ch:
			switch s {
			case syscall.SIGUSR1:
				server.Daemon.DumpInfo()
			case syscall.SIGINT, syscall.SIGTERM:
				server.Close()
				return
			default:
				log.Warnw("uncaught signal", "signal", s)
			}
		case <-server.ctx.Done():
			server.Close()
			return
		}
	}
}

func parse_address(s string) (string, string) {
	if strings.HasPrefix(s, "unix:") {
		ss := strings.SplitN(s, ":", 2)
		return "unix", ss[1]
	}
	return "tcp", s
}

func (server *Server) clearUnixSockets() error {
	if server.ListenNetwork != "unix" {
		return nil
	}

	if err := os.Remove(server.ListenAddr); err != nil {
		return err
	}

	return nil
}

func (server *Server) Close() {
	var merr *multierror.Error
	if err := server.Listener.Close(); err != nil {
		merr = multierror.Append(err)
	}
	if err := server.clearUnixSockets(); err != nil {
		merr = multierror.Append(merr, err)
	}
	if err := server.Daemon.Close(); err != nil {
		merr = multierror.Append(err)
	}
	server.done <- struct{}{}
	merr.ErrorOrNil()
}

func newDaemonFromConfig(ctx context.Context, c Config, extra_opts ...libp2p.Option) (*daemon.Daemon, error) {
	opts := []libp2p.Option{
		libp2p.UserAgent("gop2ppipes/0.1"),
		libp2p.DefaultTransports,
	}
	if c.Muxer == "yamux" {
		opts = append(opts, libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport))
	} else if c.Muxer != "" {
		return nil, fmt.Errorf("Unsupported muxer %s", c.Muxer)
	}

	// collect opts
	if c.ID != "" {
		key, err := ReadIdentity(c.ID)
		if err != nil {
			log.Fatal(err)
		}

		opts = append(opts, libp2p.Identity(key))
	}

	if err := c.Validate(); err != nil {
		log.Fatal(err)
	}

	if len(c.HostAddresses) > 0 {
		log.Infow("Listening at", "addresses", c.HostAddresses)
		opts = append(opts, libp2p.ListenAddrs(c.HostAddresses...))
	}

	if len(c.AnnounceAddresses) > 0 {
		opts = append(opts, libp2p.AddrsFactory(func([]multiaddr.Multiaddr) []multiaddr.Multiaddr {
			return c.AnnounceAddresses
		}))
	}

	if c.ConnectionManager.Enabled {
		cm, err := connmgr.NewConnManager(c.ConnectionManager.LowWaterMark,
			c.ConnectionManager.HighWaterMark,
			connmgr.WithGracePeriod(c.ConnectionManager.GracePeriod),
		)
		if err != nil {
			panic(err)
		}
		opts = append(opts, libp2p.ConnectionManager(cm))
	}

	if c.NatPortMap {
		opts = append(opts, libp2p.NATPortMap())
	}

	if c.AutoNat {
		opts = append(opts, libp2p.EnableNATService())
	}

	if c.HolePunching {
		opts = append(opts, libp2p.EnableHolePunching())
	}

	if c.NoListen {
		opts = append(opts,
			libp2p.NoListenAddrs,
			// NoListenAddrs disables the relay transport
			libp2p.EnableRelay(),
		)
	}

	var securityOpts []libp2p.Option
	if c.Security.Noise {
		securityOpts = append(securityOpts, libp2p.Security(noise.ID, noise.New))
	}
	if c.Security.TLS {
		securityOpts = append(securityOpts, libp2p.Security(tls.ID, tls.New))
	}

	if len(securityOpts) == 0 {
		log.Fatal("at least one channel security protocol must be enabled")
	}
	opts = append(opts, securityOpts...)

	if c.PProf.Enabled {
		// an invalid port number will fail within the function.
		go pprofHTTP(int(c.PProf.Port))
	}

	if len(c.Relay.Auto.Peers) > 0 {
		c.Relay.Auto.Enabled = true
		log.Infow("Auto relay peers", "peers", c.Relay.Auto.Peers)
		opts = append(opts, libp2p.EnableAutoRelayWithStaticRelays(c.Relay.Auto.Peers))
	}

	opts = append(opts, extra_opts...)

	ticker := backoff.NewTicker(&backoff.ExponentialBackOff{
		InitialInterval:     5 * time.Second,
		RandomizationFactor: 0.5,
		Multiplier:          2,
		MaxInterval:         60 * time.Minute,
		MaxElapsedTime:      0,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	})
	daemonConfig := daemon.DaemonConfig{
		DHTMode:            c.DHT.Mode,
		DHTBroadcastTicker: ticker,
	}
	d, err := daemon.NewDaemon(ctx, daemonConfig, opts...)
	if err != nil {
		return nil, err
	}

	if c.PubSub.Enabled {
		if c.PubSub.GossipSubHeartbeat.Interval > 0 {
			ps.GossipSubHeartbeatInterval = c.PubSub.GossipSubHeartbeat.Interval
		}
		if c.PubSub.GossipSubHeartbeat.InitialDelay > 0 {
			ps.GossipSubHeartbeatInitialDelay = c.PubSub.GossipSubHeartbeat.InitialDelay
		}

		err = d.EnablePubsub(c.PubSub.Router, c.PubSub.Sign, c.PubSub.SignStrict)
		if err != nil {
			log.Fatal(err)
		}
	}

	if c.Relay.Enabled {
		err = d.EnableRelayV2()
		if err != nil {
			log.Fatal(err)
		}
	}

	if len(c.Bootstrap.Peers) == 0 && c.Bootstrap.UseDefaultPeers {
		c.Bootstrap.Peers = dht.DefaultBootstrapPeers
	}

	log.Infow("Starting daemon", "Control socket", c.ListenAddr, "Peer ID", d.ID().Pretty(), "Addrs", d.Host.Addrs())
	if c.Bootstrap.Enabled && len(c.Bootstrap.Peers) > 0 {
		log.Infow("Bootstrapping peers", "peers", c.Bootstrap.Peers)
		err = d.Bootstrap(c.Bootstrap.Peers)
		if err != nil {
			log.Errorw("Bootstrap failed", "error", err)
		}
	}

	if c.MetricsAddress != "" {
		http.Handle("/metrics", promhttp.Handler())
		go func() { http.ListenAndServe(c.MetricsAddress, nil) }()
	}
	return d, nil
}

func pprofHTTP(port int) {
	listen := func(p int) error {
		addr := fmt.Sprintf("localhost:%d", p)
		log.Infow("registering pprof debug http handle", "addr", fmt.Sprintf("http://%s/debug/pprof/\n", addr))
		switch err := http.ListenAndServe(addr, nil); err {
		case nil:
			// all good, server is running and exited normally.
			return nil
		case http.ErrServerClosed:
			// all good, server was shut down.
			return nil
		default:
			// error, try another port
			log.Errorf("error registering pprof debug http handler at: %s: %s\n", addr, err)
			return err
		}
	}

	if port > 0 {
		// we have a user-assigned port.
		_ = listen(port)
		return
	}

	// we don't have a user assigned port, try sequentially to bind between [6060-7080]
	for i := 6060; i <= 7080; i++ {
		if listen(i) == nil {
			return
		}
	}
}
