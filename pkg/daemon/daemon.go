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
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	ps "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	multierror "github.com/hashicorp/go-multierror"
	logging "github.com/ipfs/go-log"
	config "github.com/libp2p/go-libp2p-daemon/config"

	_ "net/http/pprof"
)

var log = logging.Logger("daemon")

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
