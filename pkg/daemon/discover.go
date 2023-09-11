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

func (d *Daemon) FindDHTPeersAsync(ctx context.Context, rdv string, count int) (<-chan peer.AddrInfo, error) {
	dht := d.DHT()
	if dht == nil {
		return nil, ERROR_NO_DHT
	}
	log.Infow("Find via dht", "dht", d.DHT())
	cid := d.getRendezvousCid(rdv)
	err := d.DHT().Provide(ctx, cid, true)
	if err != nil {
		log.Warnw("DHT error while providing", "rdv", rdv, "cid", cid, "error", err)
		return nil, err
	}
	return d.DHT().FindProvidersAsync(ctx, d.getRendezvousCid(rdv), defaultProviderCount), nil
}
