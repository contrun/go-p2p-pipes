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

	logging "github.com/ipfs/go-log"

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

func (d *Daemon) DumpInfo() {
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
	if d.closed {
		d.mx.Unlock()
		return nil
	}
	d.closed = true
	defer d.mx.Unlock()

	return d.Host.Close()
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

	return d, nil
}
