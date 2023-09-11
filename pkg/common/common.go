package common

import (
	"os"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

func maybeRemoveUnixListenerPath(ma multiaddr.Multiaddr) error {
	c, _ := multiaddr.SplitFirst(ma)
	if c.Protocol().Code == multiaddr.P_UNIX {
		return os.Remove(c.Value())
	}
	return nil
}

// The same as closing listener except when the listener listens
// to a unix domain socket. In that case, the file associated to
// the socket is also removed.
func CloseMaNetListener(l manet.Listener) error {
	var merr *multierror.Error
	var ma = l.Multiaddr()

	if err := l.Close(); err != nil {
		merr = multierror.Append(err)
	}

	if err := maybeRemoveUnixListenerPath(ma); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr.ErrorOrNil()
}
