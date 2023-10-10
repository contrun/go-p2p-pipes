package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

func (d *Daemon) getRendezvousPoint(where string) string {
	return "gop2ppipes:" + d.namespace + ":" + where
}

func (d *Daemon) dhtMaintenance(ctx context.Context) error {
	log.Debug("Doing DHT maintenance")

	var merr *multierror.Error

	err := d.AdvertiseAllViaDHT(ctx)
	if err != nil {
		log.Warnw("Advertising via DHT error", "error", err)
		merr = multierror.Append(merr, err)
	}

	peers, err := d.FindPeersAllViaDHT(ctx)
	if err != nil {
		log.Warnw("Finding peers via DHT error", "error", err)
		merr = multierror.Append(merr, err)
	}

	for p := range peers {
		log.Debugw("Found peer via DHT", "peer", p)
		// Don't dial ourselves or peers without address
		if p.ID == d.ID() || len(p.Addrs) == 0 {
			continue
		}

		if d.Network().Connectedness(p.ID) != network.Connected {
			log.Debugw("Found new peer", "peer", p)
			if err := d.Connect(ctx, p); err != nil {
				log.Debugw("Connecting to peer failed", "peer", p, "error", err)
				merr = multierror.Append(merr, err)
			} else {
				log.Debugw("Connected to peer", "peer", p)
			}
		} else {
			log.Debugw("Found an already connected peer", "peer", p)
		}
	}

	return merr.ErrorOrNil()
}

func (d *Daemon) dhtBackgroundWorker(ticker *backoff.Ticker) {
	if d.DHT() == nil {
		return
	}

	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := d.dhtMaintenance(d.ctx)
			if err != nil {
				log.Warnw("Error while doing dht maintenance", "error", err)
			}
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

func (d *Daemon) GetCurrentDHTRendezvous() []string {
	d.dhtMx.RLock()
	defer d.dhtMx.RUnlock()
	rvs := make([]string, len(d.dhtRendezvous))
	for rv := range d.dhtRendezvous {
		rvs = append(rvs, rv)
	}
	return rvs
}

func (d *Daemon) AdvertiseAllViaDHT(ctx context.Context) error {
	var merr *multierror.Error
	var rvs = d.GetCurrentDHTRendezvous()
	for _, rv := range rvs {
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

// This function may return both an error and a non-empty channel
func (d *Daemon) FindPeersAllViaDHT(ctx context.Context) (<-chan peer.AddrInfo, error) {
	var merr *multierror.Error
	var rvs = d.GetCurrentDHTRendezvous()
	var ps = make(chan peer.AddrInfo, 100)
	var wg sync.WaitGroup

	for _, rv := range rvs {
		if peers, err := d.FindPeersViaDHT(d.ctx, rv); err != nil {
			merr = multierror.Append(err)
		} else {
			wg.Add(1)
			// Avoid blocking when the receiver of the channel is not ready
			go func() {
				for p := range peers {
					ps <- p
				}
				wg.Done()
			}()
		}
	}

	go func() {
		wg.Wait()
		close(ps)
	}()
	return ps, merr.ErrorOrNil()
}

func (d *Daemon) FindPeersViaDHT(ctx context.Context, rv string) (<-chan peer.AddrInfo, error) {
	log.Infow("Finding peers via dht", "dht", d.DHT(), "rendezvous", rv)
	return d.findPeersViaDHT(ctx, d.getRendezvousPoint(rv))
}

func (d *Daemon) FindPeersViaDHTSync(ctx context.Context, rv string) ([]peer.AddrInfo, error) {
	peersCh, err := d.FindPeersViaDHT(ctx, rv)
	if err != nil {
		return nil, err
	}
	peers := make([]peer.AddrInfo, 0)
	for peer := range peersCh {
		if peer.ID == d.Host.ID() {
			continue
		}
		peers = append(peers, peer)
	}
	return peers, nil
}
