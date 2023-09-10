package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	ps "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	multierror "github.com/hashicorp/go-multierror"
	logging "github.com/ipfs/go-log"
	config "github.com/libp2p/go-libp2p-daemon/config"

	"net/http"
	_ "net/http/pprof"
)

var log = logging.Logger("daemon")

type Daemon struct {
	host.Host
	ctx      context.Context
	listener manet.Listener

	dht    *dht.IpfsDHT
	pubsub *ps.PubSub

	mx sync.Mutex
	// stream handlers: map of protocol.ID to multi-address
	handlers map[protocol.ID]ma.Multiaddr
	// closed is set when the daemon is shutting down
	closed bool
}

func (d *Daemon) DHTRoutingFactory(opts []dhtopts.Option) func(host.Host) (routing.PeerRouting, error) {
	makeRouting := func(h host.Host) (routing.PeerRouting, error) {
		dhtInst, err := dht.New(d.ctx, h, opts...)
		if err != nil {
			return nil, err
		}
		d.dht = dhtInst
		return dhtInst, nil
	}

	return makeRouting
}

func (d *Daemon) EnableRelayV2() error {
	_, err := relay.New(d.Host)
	return err
}

func (d *Daemon) EnablePubsub(router string, sign, strict bool) error {
	var opts []ps.Option

	if !sign {
		opts = append(opts, ps.WithMessageSigning(false))
	} else if !strict {
		opts = append(opts, ps.WithStrictSignatureVerification(false))
	} else {
		opts = append(opts, ps.WithMessageSignaturePolicy(ps.StrictSign))
	}

	switch router {
	case "floodsub":
		pubsub, err := ps.NewFloodSub(d.ctx, d.Host, opts...)
		if err != nil {
			return err
		}
		d.pubsub = pubsub
		return nil

	case "gossipsub":
		pubsub, err := ps.NewGossipSub(d.ctx, d.Host, opts...)
		if err != nil {
			return err
		}
		d.pubsub = pubsub
		return nil

	default:
		return fmt.Errorf("unknown pubsub router: %s", router)
	}

}

func (d *Daemon) listen() {
	for {
		if d.isClosed() {
			return
		}

		c, err := d.listener.Accept()
		if err != nil {
			log.Errorw("error accepting connection", "error", err)
			continue
		}

		log.Debug("incoming connection")
		// TODO: handle connection here
		// go d.handleConn(c)
		_ = c

	}
}

func (d *Daemon) isClosed() bool {
	d.mx.Lock()
	defer d.mx.Unlock()
	return d.closed
}

func (d *Daemon) trapSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case s := <-ch:
			switch s {
			case syscall.SIGUSR1:
				d.handleSIGUSR1()
			case syscall.SIGINT, syscall.SIGTERM:
				d.Close()
				os.Exit(0x80 + int(s.(syscall.Signal)))
			default:
				log.Warnw("uncaught signal", "signal", s)
			}
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *Daemon) handleSIGUSR1() {
	// this is our signal to dump diagnostics info.
	if d.dht != nil {
		log.Infof("DHT Routing Table:")
		d.dht.RoutingTable().Print()
	}

	conns := d.Host.Network().Conns()
	log.Infof("Connections and streams (%d):\n", len(conns))

	for _, c := range conns {
		protos, _ := d.Host.Peerstore().GetProtocols(c.RemotePeer()) // error value here is useless

		protoVersion, err := d.Host.Peerstore().Get(c.RemotePeer(), "ProtocolVersion")
		if err != nil {
			protoVersion = "(unknown)"
		}

		agent, err := d.Host.Peerstore().Get(c.RemotePeer(), "AgentVersion")
		if err != nil {
			agent = "(unknown)"
		}

		streams := c.GetStreams()
		log.Infow("Dumpping stream info", "peer", c.RemotePeer().Pretty(), "multiaddr", c.RemoteMultiaddr(),
			"protoVersion", protoVersion, "agent", agent,
			"protocols", protos,
			"streams", len(streams),
		)
		for i, s := range streams {
			log.Infow("Dumpping stream protocol", "stream index", i, "protocol", s.Protocol())
		}
	}
}

func (d *Daemon) Close() error {
	d.mx.Lock()
	d.closed = true
	d.mx.Unlock()

	var merr *multierror.Error
	if err := d.Host.Close(); err != nil {
		merr = multierror.Append(err)
	}

	listenAddr := d.listener.Multiaddr()
	if err := d.listener.Close(); err != nil {
		merr = multierror.Append(merr, err)
	}

	if err := clearUnixSockets(listenAddr); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr.ErrorOrNil()
}

func clearUnixSockets(path ma.Multiaddr) error {
	c, _ := ma.SplitFirst(path)
	if c.Protocol().Code != ma.P_UNIX {
		return nil
	}

	if err := os.Remove(c.Value()); err != nil {
		return err
	}

	return nil
}

func NewDaemon(ctx context.Context, maddr ma.Multiaddr, dhtMode string, opts ...libp2p.Option) (*Daemon, error) {
	d := &Daemon{
		ctx:      ctx,
		handlers: make(map[protocol.ID]ma.Multiaddr),
	}

	if dhtMode != "" {
		var dhtOpts []dhtopts.Option
		if dhtMode == config.DHTClientMode {
			dhtOpts = append(dhtOpts, dht.Mode(dht.ModeClient))
		} else if dhtMode == config.DHTServerMode {
			dhtOpts = append(dhtOpts, dht.Mode(dht.ModeServer))
		}

		opts = append(opts, libp2p.Routing(d.DHTRoutingFactory(dhtOpts)))
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}
	d.Host = h

	l, err := manet.Listen(maddr)
	if err != nil {
		h.Close()
		return nil, err
	}
	d.listener = l

	go d.listen()
	go d.trapSignals()

	return d, nil
}

func NewDaemonFromConfig(ctx context.Context, c Config, extra_opts ...libp2p.Option) (*Daemon, error) {
	opts := []libp2p.Option{
		libp2p.UserAgent("p2pd/0.1"),
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

	opts = append(opts, extra_opts...)

	d, err := NewDaemon(context.Background(), &c.ListenAddr, c.DHT.Mode, opts...)
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

	if len(c.Bootstrap.Peers) == 0 {
		c.Bootstrap.Peers = dht.DefaultBootstrapPeers
	}

	if c.Bootstrap.Enabled {
		err = d.Bootstrap(c.Bootstrap.Peers)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Infow("Starting daemon", "Control socket", c.ListenAddr.String(), "Peer ID", d.ID().Pretty(), "Addrs", d.Host.Addrs())
	if c.Bootstrap.Enabled && len(c.Bootstrap.Peers) > 0 {
		log.Infow("Bootstrapping peers", "peers", c.Bootstrap.Peers)
		d.Bootstrap(c.Bootstrap.Peers)
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
