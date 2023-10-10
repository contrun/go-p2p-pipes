package daemon

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

func (d *Daemon) getRendezvousPoint(where string) string {
	return "gop2ppipes:" + d.namespace + ":" + where
}

func (d *Daemon) broadcastDHTRendezvousWorker(ticker *backoff.Ticker) {
	if d.DHT() == nil {
		return
	}

	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := d.BroadcastPeerInfoViaDHT(d.ctx)
			log.Errorw("Broadcast peer info failed", "error", err)
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *Daemon) AddDHTRendezvous(ctx context.Context, rv string) error {
	d.dhtMx.Lock()
	defer d.dhtMx.Unlock()
	if d.DHT() == nil {
		return ERROR_NO_DHT
	}
	d.dhtRendezvous[rv] = true
	return d.AdvertiseViaDHT(ctx, rv)
}

// TODO: provider information of other peers may be obsolete, do we have any way
// to proactively evict us from the provider list?
func (d *Daemon) DeleteDHTRendezvous(ctx context.Context, rv string) error {
	d.dhtMx.Lock()
	defer d.dhtMx.Unlock()
	if d.DHT() == nil {
		return ERROR_NO_DHT
	}
	delete(d.dhtRendezvous, rv)
	return nil
}

func (d *Daemon) BroadcastPeerInfoViaDHT(ctx context.Context) error {
	d.dhtMx.RLock()
	rvs := d.dhtRendezvous
	d.dhtMx.RUnlock()

	var merr *multierror.Error
	for rv := range rvs {
		if err := d.AdvertiseViaDHT(d.ctx, rv); err != nil {
			merr = multierror.Append(err)
		}
	}
	return merr.ErrorOrNil()
}

func (d *Daemon) advertiseViaDHT(ctx context.Context, rv string) (duration time.Duration, err error) {
	dht := d.DHT()
	if dht == nil {
		return duration, ERROR_NO_DHT
	}
	log.Infow("Advertising via dht", "dht", d.DHT(), "rendezvous", rv)
	routingDiscovery := routing.NewRoutingDiscovery(dht)
	return routingDiscovery.Advertise(ctx, rv)
}

func (d *Daemon) AdvertiseViaDHT(ctx context.Context, rv string) error {
	ttl, err := d.advertiseViaDHT(ctx, d.getRendezvousPoint(rv))
	if err != nil {
		log.Warnw("DHT error while providing", "rendezvous", rv, "error", err)
		return err
	}
	log.Debugw("DHT Advertising successfully", "rendezvous", rv, "ttl", ttl)
	return nil
}

func (d *Daemon) findPeersViaDHT(ctx context.Context, rv string) (<-chan peer.AddrInfo, error) {
	dht := d.DHT()
	if dht == nil {
		return nil, ERROR_NO_DHT
	}
	log.Infow("Finding peers via dht", "dht", d.DHT(), "rendezvous", rv)
	routingDiscovery := routing.NewRoutingDiscovery(dht)
	return routingDiscovery.FindPeers(ctx, rv)
}

func (d *Daemon) FindPeersViaDHT(ctx context.Context, rv string, count int) (<-chan peer.AddrInfo, error) {
	log.Infow("Finding peers via dht", "dht", d.DHT(), "rendezvous", rv)
	return d.findPeersViaDHT(ctx, d.getRendezvousPoint(rv))
}

func (d *Daemon) FindPeersViaDHTSync(ctx context.Context, rv string, count int) ([]peer.AddrInfo, error) {
	peersCh, err := d.FindPeersViaDHT(ctx, rv, count)
	if err != nil {
		return nil, err
	}
	peers := make([]peer.AddrInfo, 0)
	for peer := range peersCh {
		peers = append(peers, peer)
	}
	return peers, nil
}
