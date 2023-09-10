package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	ps "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	multierror "github.com/hashicorp/go-multierror"
	logging "github.com/ipfs/go-log"
	config "github.com/libp2p/go-libp2p-daemon/config"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	connmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	noise "github.com/libp2p/go-libp2p/p2p/security/noise"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
	multiaddr "github.com/multiformats/go-multiaddr"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"

	_ "net/http/pprof"
)

var log = logging.Logger("p2pd")

func pprofHTTP(port int) {
	listen := func(p int) error {
		addr := fmt.Sprintf("localhost:%d", p)
		log.Infof("registering pprof debug http handler at: http://%s/debug/pprof/\n", addr)
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

type Daemon struct {
	ctx      context.Context
	host     host.Host
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
	_, err := relay.New(d.host)
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
		pubsub, err := ps.NewFloodSub(d.ctx, d.host, opts...)
		if err != nil {
			return err
		}
		d.pubsub = pubsub
		return nil

	case "gossipsub":
		pubsub, err := ps.NewGossipSub(d.ctx, d.host, opts...)
		if err != nil {
			return err
		}
		d.pubsub = pubsub
		return nil

	default:
		return fmt.Errorf("unknown pubsub router: %s", router)
	}

}

func (d *Daemon) ID() peer.ID {
	return d.host.ID()
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
		fmt.Println("DHT Routing Table:")
		d.dht.RoutingTable().Print()
		fmt.Println()
		fmt.Println()
	}

	conns := d.host.Network().Conns()
	fmt.Printf("Connections and streams (%d):\n", len(conns))

	for _, c := range conns {
		protos, _ := d.host.Peerstore().GetProtocols(c.RemotePeer()) // error value here is useless

		protoVersion, err := d.host.Peerstore().Get(c.RemotePeer(), "ProtocolVersion")
		if err != nil {
			protoVersion = "(unknown)"
		}

		agent, err := d.host.Peerstore().Get(c.RemotePeer(), "AgentVersion")
		if err != nil {
			agent = "(unknown)"
		}

		streams := c.GetStreams()
		fmt.Printf("peer: %s, multiaddr: %s\n", c.RemotePeer().Pretty(), c.RemoteMultiaddr())
		fmt.Printf("\tprotoVersion: %s, agent: %s\n", protoVersion, agent)
		fmt.Printf("\tprotocols: %v\n", protos)
		fmt.Printf("\tstreams (%d):\n", len(streams))
		for _, s := range streams {
			fmt.Println("\t\tprotocol: ", s.Protocol())
		}
	}
}

func (d *Daemon) Close() error {
	d.mx.Lock()
	d.closed = true
	d.mx.Unlock()

	var merr *multierror.Error
	if err := d.host.Close(); err != nil {
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
	d.host = h

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

func main() {
	maddrString := flag.String("listen", "/unix/tmp/p2pd.sock", "daemon control listen multiaddr")
	quiet := flag.Bool("q", false, "be quiet")
	id := flag.String("id", "", "peer identity; private key file")
	bootstrap := flag.Bool("b", false, "connects to bootstrap peers and bootstraps the dht if enabled")
	bootstrapPeers := flag.String("bootstrapPeers", "", "comma separated list of bootstrap peers; defaults to the IPFS DHT peers")
	enableDht := flag.Bool("dht", false, "Enables the DHT in full node mode")
	dhtClient := flag.Bool("dhtClient", false, "Enables the DHT in client mode")
	dhtServer := flag.Bool("dhtServer", false, "Enables the DHT in server mode (use 'dht' unless you actually need this)")
	connMgr := flag.Bool("connManager", false, "Enables the Connection Manager")
	connMgrLo := flag.Int("connLo", 256, "Connection Manager Low Water mark")
	connMgrHi := flag.Int("connHi", 512, "Connection Manager High Water mark")
	connMgrGrace := flag.Duration("connGrace", 120*time.Second, "Connection Manager grace period (in seconds)")
	natPortMap := flag.Bool("natPortMap", false, "Enables NAT port mapping")
	pubsub := flag.Bool("pubsub", false, "Enables pubsub")
	pubsubRouter := flag.String("pubsubRouter", "gossipsub", "Specifies the pubsub router implementation")
	pubsubSign := flag.Bool("pubsubSign", true, "Enables pubsub message signing")
	pubsubSignStrict := flag.Bool("pubsubSignStrict", true, "Enables or disables pubsub strict signature verification")
	gossipsubHeartbeatInterval := flag.Duration("gossipsubHeartbeatInterval", 0, "Specifies the gossipsub heartbeat interval")
	gossipsubHeartbeatInitialDelay := flag.Duration("gossipsubHeartbeatInitialDelay", 0, "Specifies the gossipsub initial heartbeat delay")
	relayEnabled := flag.Bool("relay", true, "Enables circuit relay")
	relayActive := flag.Bool("relayActive", false, "Enables active mode for relay")
	relayHop := flag.Bool("relayHop", false, "Enables hop for relay")
	relayHopLimit := flag.Int("relayHopLimit", 0, "Sets the hop limit for hop relays")
	relayDiscovery := flag.Bool("relayDiscovery", false, "Enables passive discovery for relay")
	autoRelay := flag.Bool("autoRelay", false, "Enables autorelay")
	autonat := flag.Bool("autonat", false, "Enables the AutoNAT service")
	hostAddrs := flag.String("hostAddrs", "", "comma separated list of multiaddrs the host should listen on")
	announceAddrs := flag.String("announceAddrs", "", "comma separated list of multiaddrs the host should announce to the network")
	noListen := flag.Bool("noListenAddrs", false, "sets the host to listen on no addresses")
	metricsAddr := flag.String("metricsAddr", "", "an address to bind the metrics handler to")
	configFilename := flag.String("f", "", "a file from which to read a json representation of the deamon config")
	configStdin := flag.Bool("i", false, "have the daemon read the json config from stdin")
	pprof := flag.Bool("pprof", false, "Enables the HTTP pprof handler, listening on the first port "+
		"available in the range [6060-7800], or on the user-provided port via -pprofPort")
	pprofPort := flag.Uint("pprofPort", 0, "Binds the HTTP pprof handler to a specific port; "+
		"has no effect unless the pprof option is enabled")
	useNoise := flag.Bool("noise", true, "Enables Noise channel security protocol")
	useTls := flag.Bool("tls", true, "Enables TLS1.3 channel security protocol")
	forceReachabilityPublic := flag.Bool("forceReachabilityPublic", false, "Set up ForceReachability as public for autonat")
	forceReachabilityPrivate := flag.Bool("forceReachabilityPrivate", false, "Set up ForceReachability as private for autonat")
	muxer := flag.String("muxer", "yamux", "muxer to use for connections")

	flag.Parse()

	var c config.Config
	opts := []libp2p.Option{
		libp2p.UserAgent("p2pd/0.1"),
		libp2p.DefaultTransports,
	}

	if *configStdin {
		stdin := bufio.NewReader(os.Stdin)
		body, err := io.ReadAll(stdin)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(body, &c); err != nil {
			log.Fatal(err)
		}
	} else if *configFilename != "" {
		body, err := os.ReadFile(*configFilename)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(body, &c); err != nil {
			log.Fatal(err)
		}
	} else {
		c = config.NewDefaultConfig()
	}

	maddr, err := multiaddr.NewMultiaddr(*maddrString)
	if err != nil {
		log.Fatal(err)
	}
	c.ListenAddr = config.JSONMaddr{Multiaddr: maddr}

	if *id != "" {
		c.ID = *id
	}

	if *hostAddrs != "" {
		addrStrings := strings.Split(*hostAddrs, ",")
		ha := make([]multiaddr.Multiaddr, len(addrStrings))
		for i, s := range addrStrings {
			ma, err := multiaddr.NewMultiaddr(s)
			if err != nil {
				log.Fatal(err)
			}
			(ha)[i] = ma
		}
		c.HostAddresses = ha
	}

	if *announceAddrs != "" {
		addrStrings := strings.Split(*announceAddrs, ",")
		ha := make([]multiaddr.Multiaddr, len(addrStrings))
		for i, s := range addrStrings {
			ma, err := multiaddr.NewMultiaddr(s)
			if err != nil {
				log.Fatal(err)
			}
			(ha)[i] = ma
		}
		c.AnnounceAddresses = ha
	}

	if *connMgr {
		c.ConnectionManager.Enabled = true
		c.ConnectionManager.GracePeriod = *connMgrGrace
		c.ConnectionManager.HighWaterMark = *connMgrHi
		c.ConnectionManager.LowWaterMark = *connMgrLo
	}

	if *natPortMap {
		c.NatPortMap = true
	}

	if *relayEnabled {
		c.Relay.Enabled = true
		if *relayActive {
			c.Relay.Active = true
		}
		if *relayHop {
			c.Relay.Hop = true
		}
		if *relayDiscovery {
			c.Relay.Discovery = true
		}
		if *relayHopLimit > 0 {
			c.Relay.HopLimit = *relayHopLimit
		}
	}

	if *autoRelay {
		c.Relay.Auto = true
	}

	if *noListen {
		c.NoListen = true
	}

	if *autonat {
		c.AutoNat = true
	}

	if *pubsub {
		c.PubSub.Enabled = true
		c.PubSub.Router = *pubsubRouter
		c.PubSub.Sign = *pubsubSign
		c.PubSub.SignStrict = *pubsubSignStrict
		if *gossipsubHeartbeatInterval > 0 {
			c.PubSub.GossipSubHeartbeat.Interval = *gossipsubHeartbeatInterval
		}
		if *gossipsubHeartbeatInitialDelay > 0 {
			c.PubSub.GossipSubHeartbeat.InitialDelay = *gossipsubHeartbeatInitialDelay
		}
	}

	if *bootstrapPeers != "" {
		addrStrings := strings.Split(*bootstrapPeers, ",")
		bps := make([]multiaddr.Multiaddr, len(addrStrings))
		for i, s := range addrStrings {
			ma, err := multiaddr.NewMultiaddr(s)
			if err != nil {
				log.Fatal(err)
			}
			(bps)[i] = ma
		}
		c.Bootstrap.Peers = bps
	}

	if *bootstrap {
		c.Bootstrap.Enabled = true
	}

	if *muxer == "yamux" {
		opts = append(opts, libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport))
	}

	if *quiet {
		c.Quiet = true
	}

	if *metricsAddr != "" {
		c.MetricsAddress = *metricsAddr
	}

	if *enableDht {
		c.DHT.Mode = config.DHTFullMode
	} else if *dhtClient {
		c.DHT.Mode = config.DHTClientMode
	} else if *dhtServer {
		c.DHT.Mode = config.DHTServerMode
	}

	if *pprof {
		c.PProf.Enabled = true
		if pprofPort != nil {
			c.PProf.Port = *pprofPort
		}
	}

	if useTls != nil {
		c.Security.TLS = *useTls
	}
	if useNoise != nil {
		c.Security.Noise = *useNoise
	}

	if err := c.Validate(); err != nil {
		log.Fatal(err)
	}

	if c.PProf.Enabled {
		// an invalid port number will fail within the function.
		go pprofHTTP(int(c.PProf.Port))
	}

	// collect opts
	if c.ID != "" {
		key, err := ReadIdentity(c.ID)
		if err != nil {
			log.Fatal(err)
		}

		opts = append(opts, libp2p.Identity(key))
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

	if *forceReachabilityPrivate && *forceReachabilityPublic {
		log.Fatal("forceReachability must be public or private, not both")
	} else if *forceReachabilityPrivate {
		opts = append(opts, libp2p.ForceReachabilityPrivate())
	} else if *forceReachabilityPublic {
		opts = append(opts, libp2p.ForceReachabilityPublic())
	}

	// start daemon
	d, err := NewDaemon(context.Background(), &c.ListenAddr, c.DHT.Mode, opts...)
	if err != nil {
		log.Fatal(err)
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

	if !c.Quiet {
		fmt.Printf("Control socket: %s\n", c.ListenAddr.String())
		fmt.Printf("Peer ID: %s\n", d.ID().Pretty())
		fmt.Printf("Peer Addrs:\n")
		// for _, addr := range d.Addrs() {
		// 	fmt.Printf("%s\n", addr.String())
		// }
		if c.Bootstrap.Enabled && len(c.Bootstrap.Peers) > 0 {
			fmt.Printf("Bootstrap peers:\n")
			for _, p := range c.Bootstrap.Peers {
				fmt.Printf("%s\n", p)
			}
		}
	}

	if c.MetricsAddress != "" {
		http.Handle("/metrics", promhttp.Handler())
		go func() { http.ListenAndServe(c.MetricsAddress, nil) }()
	}

	select {}
}
