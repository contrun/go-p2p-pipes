package daemon

import (
	"github.com/ipfs/go-cid"
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

func (d *Daemon) RendezvousAt(point string) error {
	return nil
}
