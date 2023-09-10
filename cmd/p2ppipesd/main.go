package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"time"

	"github.com/contrun/go-p2p-pipes/pkg/daemon"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("p2ppipesd")

func main() {
	loglevel := os.Getenv("GOLOG_LOG_LEVEL")
	if loglevel == "" {
		logging.SetLogLevel("*", "info")
	}

	maddrString := flag.String("listen", "/unix/tmp/p2pd.sock", "daemon control listen multiaddr")
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

	var c daemon.Config

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
		c = daemon.NewDefaultConfig()
	}

	maddr, err := multiaddr.NewMultiaddr(*maddrString)
	if err != nil {
		log.Fatal(err)
	}
	c.ListenAddr = daemon.JSONMaddr{Multiaddr: maddr}

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

	c.Muxer = *muxer

	if *metricsAddr != "" {
		c.MetricsAddress = *metricsAddr
	}

	if *enableDht {
		c.DHT.Mode = daemon.DHTFullMode
	} else if *dhtClient {
		c.DHT.Mode = daemon.DHTClientMode
	} else if *dhtServer {
		c.DHT.Mode = daemon.DHTServerMode
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

	opts := []libp2p.Option{}
	if *forceReachabilityPrivate && *forceReachabilityPublic {
		log.Fatal("forceReachability must be public or private, not both")
	} else if *forceReachabilityPrivate {
		opts = append(opts, libp2p.ForceReachabilityPrivate())
	} else if *forceReachabilityPublic {
		opts = append(opts, libp2p.ForceReachabilityPublic())
	}

	// start daemon
	_, err = daemon.NewDaemonFromConfig(context.Background(), c, opts...)
	if err != nil {
		log.Fatal(err)
	}

	select {}
}
