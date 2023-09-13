package daemon

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
)

const defaultProviderCount = 20

func (d *Daemon) getRendezvousCid(where string) cid.Cid {
	rdv := []byte(where)
	rdv = append(rdv, []byte(":gop2ppipes"+d.namespace)...)
	hash, err := multihash.Sum(rdv, multihash.SHA2_256, -1)
	if err != nil {
		panic(err)
	}
	id := cid.NewCidV1(cid.Raw, hash)
	log.Debugw("Rendezvous point", "id", id)
	return id
}

func (d *Daemon) BroadcastPeerInfoViaDHT(ctx context.Context, rv string) error {
	dht := d.DHT()
	if dht == nil {
		return ERROR_NO_DHT
	}
	cid := d.getRendezvousCid(rv)
	log.Infow("Broadcasting peer info via dht", "dht", d.DHT(), "rendezvous", rv, "cid", cid)
	err := d.DHT().Provide(ctx, cid, true)
	if err != nil {
		log.Warnw("DHT error while providing", "rendezvous", rv, "cid", cid, "error", err)
		return err
	}
	return nil
}

func (d *Daemon) FindPeersViaDHT(ctx context.Context, rv string, count int) (<-chan peer.AddrInfo, error) {
	dht := d.DHT()
	if dht == nil {
		return nil, ERROR_NO_DHT
	}
	cid := d.getRendezvousCid(rv)
	log.Infow("Finding peers via dht", "dht", d.DHT(), "rendezvous", rv, "cid", cid)
	return d.DHT().FindProvidersAsync(ctx, cid, defaultProviderCount), nil
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
