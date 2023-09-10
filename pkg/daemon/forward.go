package daemon

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"

	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var ForwardingProtocolID = protocol.ID("/gop2ppipes/forward/v0.1.0")

type ForwardingRequest struct {
	// TODO: we should obtain the peer id from rpc the request connection
	// not from rpc payload set by the peer
	ID      string
	Address string
}

type ForwardingResponse struct{}

type ForwardingService struct {
	daemon *Daemon
}

func (d *ForwardingService) Forward(ctx context.Context, request ForwardingRequest, response *ForwardingResponse) error {
	log.Info("Received a Forwarding call")
	peer, err := peer.Decode(request.ID)
	if err != nil {
		return err
	}
	addr, err := multiaddr.NewMultiaddr(request.Address)
	if err != nil {
		return err
	}
	protocolID := d.daemon.GetForwardingProtocolID(peer, addr)
	d.daemon.SetStreamHandler(protocolID, d.daemon.ForwardStreamHandler(protocolID, peer, addr))
	return nil
}

func (d *Daemon) RegisterForwardingService() error {
	rpcHost := gorpc.NewServer(d.Host, ForwardingProtocolID)
	svc := ForwardingService{d}
	return rpcHost.Register(&svc)
}

func (d *Daemon) GetForwardingProtocolID(peer peer.ID, addr multiaddr.Multiaddr) protocol.ID {
	return protocol.ID(peer.String() + "/" + addr.String())
}

func IsStreamAuthorized(s network.Stream) bool {
	log.Debugw("Check stream authorization", "protocol", s.Protocol(), "local peer", s.Conn().LocalPeer(), "remote peer", s.Conn().RemotePeer())
	return strings.HasPrefix(string(s.Protocol()), s.Conn().RemotePeer().String()+"/")
}

func (d *Daemon) doStreamPipe(c net.Conn, s network.Stream) {
	var wg sync.WaitGroup
	wg.Add(2)

	pipe := func(dst io.WriteCloser, src io.Reader) {
		_, err := io.Copy(dst, src)
		if err != nil && err != io.EOF {
			log.Debugw("stream error", "error", err)
			s.Reset()
		}
		dst.Close()
		wg.Done()
	}

	go pipe(c, s)
	go pipe(s, c)

	wg.Wait()
}

func (d *Daemon) ForwardStreamHandler(protocol protocol.ID, peer peer.ID, localAddr multiaddr.Multiaddr) network.StreamHandler {
	f := func(s network.Stream) {
		if !IsStreamAuthorized(s) {
			log.Debugf("Reset unauthorized stream", "id", s.ID())
			s.Reset()
		}

		c, err := manet.Dial(localAddr)
		if err != nil {
			log.Debugw("Error dialing in server", "addr", localAddr.String(), "error", err)
			s.Reset()
			return
		}
		defer c.Close()
		defer d.RemoveStreamHandler(protocol)
		d.doStreamPipe(c, s)
	}

	return f
}

func (d *Daemon) RequestForwarding(peer peer.ID, remoteAddr multiaddr.Multiaddr, localAddr multiaddr.Multiaddr) error {
	// TODO: maybe with proactively clean up resources? or back pressue
	rpcClient := gorpc.NewClient(d.Host, ForwardingProtocolID)
	request := ForwardingRequest{
		ID:      d.ID().String(),
		Address: remoteAddr.String(),
	}
	var response ForwardingResponse
	err := rpcClient.Call(peer, "ForwardingService", "Forward", request, &response)
	if err != nil {
		return err
	}
	// TODO: In case some malicious user request to open a forwarding stream,
	// But it didn't actually connect to that stream. The server may register too many
	// stream hanlders. We need to clean that up.
	ctx := context.Background()
	protocolID := d.GetForwardingProtocolID(peer, remoteAddr)
	s, err := d.NewStream(ctx, peer, protocolID)
	if err != nil {
		return err
	}
	c, err := manet.Dial(localAddr)
	if err != nil {
		log.Debugw("Error dialing to local address", "addr", localAddr.String(), "error", err)
		s.Reset()
		return err
	}
	defer c.Close()
	d.doStreamPipe(c, s)
	return nil
}
