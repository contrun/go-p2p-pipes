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

var ForwardingProtocolID = protocol.ID("/gop2ppipes/forward/0.1.0")

type SetupForwardingRequest struct {
	// TODO: we should obtain the peer id from rpc the request connection
	// not from rpc payload set by the peer
	ID      string
	Address string
}

type SetupForwardingResponse struct{}

type ForwardingService struct {
	daemon *Daemon
}

// A RPC request to setup a protocol handler which will forward traffic back and forth between
// a libp2p stream and a connection the multiaddr given in the request (which will be started on demand)
// After this RPC has successfully returned, the caller may initiate a new stream
// to the receiver of this RPC which effectively connects to the given multiaddr.
func (d *ForwardingService) SetupForwarding(ctx context.Context, request SetupForwardingRequest, response *SetupForwardingResponse) error {
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
	streamHandler := func(s network.Stream) {
		daemon := d.daemon
		if !IsForwardingStreamAuthorized(s) {
			s.Reset()
			log.Debugf("Resetted unauthorized stream", "id", s.ID())
			return
		}

		c, err := manet.Dial(addr)
		if err != nil {
			log.Debugw("Error dialing in server", "addr", addr.String(), "error", err)
			s.Reset()
			return
		}
		defer c.Close()
		defer daemon.RemoveStreamHandler(protocolID)
		daemon.doStreamPipe(c, s)
	}

	d.daemon.SetStreamHandler(protocolID, streamHandler)
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

func IsForwardingStreamAuthorized(s network.Stream) bool {
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

// Listen to the local multiaddr and forward any traffic from the first
// connection to this address to the remote peer.
// Remote peer would then forward the traffic to the remote address.
// The listener will be closed after this forwarding has finished.
func (d *Daemon) ForwardTraffic(peer peer.ID, remoteAddr multiaddr.Multiaddr, localAddr multiaddr.Multiaddr) error {
	// TODO: maybe with proactively clean up resources? or back pressue
	rpcClient := gorpc.NewClient(d.Host, ForwardingProtocolID)
	request := SetupForwardingRequest{
		ID:      d.ID().String(),
		Address: remoteAddr.String(),
	}
	var response SetupForwardingResponse
	err := rpcClient.Call(peer, "ForwardingService", "SetupForwarding", request, &response)
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
		log.Error("Error while starting new stream to peer", "peer", peer, "protocol", protocolID, "error", err)
		return err
	}
	// TODO: do/can we still need to close the stream after it is resetted?
	defer s.Close()
	listener, err := manet.Listen(localAddr)
	if err != nil {
		log.Error("Error while listening to local address", "addr", localAddr.String(), "error", err)
		s.Reset()
		return err
	}
	defer listener.Close()
	c, err := listener.Accept()
	if err != nil {
		log.Debugw("Error while accepting connection to local address", "addr", localAddr.String(), "error", err)
		s.Reset()
		return err
	}
	defer c.Close()
	d.doStreamPipe(c, s)
	return nil
}
