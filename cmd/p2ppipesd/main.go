package main

import (
	"context"

	"github.com/alexflint/go-arg"
	"github.com/contrun/go-p2p-pipes/pkg/daemon"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("p2ppipesd")

type arguments struct {
	ListeningAddr            string
	ID                       string
	Bootstrap                bool `default:"true"`
	BootstrapPeers           []string
	EnableDht                bool `default:"false"`
	DhtClient                bool `default:"false"`
	DhtServer                bool `default:"false"`
	NatPortMap               bool `default:"false"`
	RelayEnabled             bool `default:"true"`
	RelayActive              bool `default:"false"`
	RelayHop                 bool `default:"false"`
	RelayHopLimit            int  `default:"0"`
	RelayDiscovery           bool `default:"false"`
	AutoRelay                bool `default:"false"`
	AutoNat                  bool `default:"false"`
	HostAddrs                []string
	AnnounceAddrs            []string
	MetricsAddr              string
	Pprof                    bool
	PprofPort                int  `default:"0"`
	UseNoise                 bool `default:"true"`
	UseTLS                   bool `default:"true"`
	ForceReachabilityPublic  bool `default:"false"`
	ForceReachabilityPrivate bool `default:"false"`
	Muxer                    string
}

func MustGetMultiaddr(s string) multiaddr.Multiaddr {
	maddr, err := multiaddr.NewMultiaddr(s)
	if err != nil {
		log.Fatal(err)
	}
	return maddr
}

func MustGetMultiaddrs(ss []string) (maddrs []multiaddr.Multiaddr) {
	for _, s := range ss {
		maddr, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			log.Fatal(err)
		}
		maddrs = append(maddrs, maddr)
	}
	return
}

func getConfig(a *arguments) daemon.Config {
	c := daemon.NewDefaultConfig()

	if a.ListeningAddr != "" {
		c.ListenAddr = daemon.JSONMaddr{Multiaddr: MustGetMultiaddr(a.ListeningAddr)}
	}
	c.Bootstrap.Peers = MustGetMultiaddrs(a.BootstrapPeers)

	if a.ID != "" {
		c.ID = a.ID
	}

	if len(a.HostAddrs) != 0 {
		c.HostAddresses = MustGetMultiaddrs(a.HostAddrs)
	}

	if len(a.AnnounceAddrs) != 0 {
		c.AnnounceAddresses = MustGetMultiaddrs(a.AnnounceAddrs)
	}

	c.NatPortMap = a.NatPortMap

	c.Relay.Enabled = a.RelayEnabled
	c.Relay.Active = a.RelayActive
	c.Relay.Hop = a.RelayHop
	c.Relay.Discovery = a.RelayDiscovery
	c.Relay.HopLimit = a.RelayHopLimit

	c.Relay.Auto = a.AutoRelay
	c.AutoNat = a.AutoNat

	if len(a.BootstrapPeers) != 0 {
		c.Bootstrap.Peers = MustGetMultiaddrs(a.BootstrapPeers)
	}
	c.Bootstrap.Enabled = a.Bootstrap

	c.Muxer = a.Muxer
	if a.MetricsAddr != "" {
		c.MetricsAddress = a.MetricsAddr
	}

	if a.EnableDht {
		c.DHT.Mode = daemon.DHTFullMode
	} else if a.DhtClient {
		c.DHT.Mode = daemon.DHTClientMode
	} else if a.DhtServer {
		c.DHT.Mode = daemon.DHTServerMode
	}
	if a.Pprof {
		c.PProf.Enabled = true
		if a.PprofPort != 0 {
			c.PProf.Port = uint(a.PprofPort)
		}
	}

	c.Security.TLS = a.UseTLS
	c.Security.Noise = a.UseNoise
	return c
}

func run() error {
	var a arguments
	arg.MustParse(&a)
	c := getConfig(&a)

	opts := []libp2p.Option{}
	if a.ForceReachabilityPrivate && a.ForceReachabilityPublic {
		log.Fatal("forceReachability must be public or private, not both")
	} else if a.ForceReachabilityPrivate {
		opts = append(opts, libp2p.ForceReachabilityPrivate())
	} else if a.ForceReachabilityPublic {
		opts = append(opts, libp2p.ForceReachabilityPublic())
	}

	// start daemon
	_, err := daemon.NewDaemonFromConfig(context.Background(), c, opts...)
	if err != nil {
		log.Fatal(err)
	}

	select {}
}

func main() {
	run()
}
