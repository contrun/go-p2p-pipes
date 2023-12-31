package daemon

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-daemon/config"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	ps "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/cenkalti/backoff/v4"
	logging "github.com/ipfs/go-log"

	_ "net/http/pprof"
)

var log = logging.Logger("daemon")

var ERROR_NO_DHT = fmt.Errorf("DHT not available")

type Daemon struct {
	host.Host
	namespace string
	ctx       context.Context
	listener  manet.Listener

	dht           *dht.IpfsDHT
	dhtMx         sync.RWMutex
	dhtRendezvous map[string]bool

	pubsub *ps.PubSub

	// stream handlers: map of protocol.ID to multi-address
	handlers map[protocol.ID]ma.Multiaddr
}

type DaemonConfig struct {
	DHTMode            string
	DHTBroadcastTicker *backoff.Ticker
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

func (d *Daemon) DHT() *dht.IpfsDHT {
	return d.dht
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

func (d *Daemon) DumpInfo() {
	// this is our signal to dump diagnostics info.
	if d.dht != nil {
		log.Infof("DHT Routing Table:")
		d.dht.RoutingTable().Print()
	}

	ps := d.Host.Peerstore()
	ids := ps.Peers()
	log.Infow("Dumping peer store", "#peers", len(ids))
	for _, id := range ids {
		log.Infow("Dumpping peer info", "peer info", ps.PeerInfo(id))
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
	return d.Host.Close()
}

func NewDaemon(ctx context.Context, daemonConfig DaemonConfig, opts ...libp2p.Option) (*Daemon, error) {
	d := &Daemon{
		ctx:           ctx,
		dhtRendezvous: make(map[string]bool),
	}

	if daemonConfig.DHTMode != "" {
		var dhtOpts []dhtopts.Option
		if daemonConfig.DHTMode == config.DHTClientMode {
			dhtOpts = append(dhtOpts, dht.Mode(dht.ModeClient))
		} else if daemonConfig.DHTMode == config.DHTServerMode {
			dhtOpts = append(dhtOpts, dht.Mode(dht.ModeServer))
		}

		opts = append(opts, libp2p.Routing(d.DHTRoutingFactory(dhtOpts)))
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}
	d.Host = h

	d.RegisterForwardingService()
	go d.dhtBackgroundWorker(daemonConfig.DHTBroadcastTicker)
	return d, nil
}
