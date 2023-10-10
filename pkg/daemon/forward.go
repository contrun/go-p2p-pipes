package daemon

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/contrun/go-p2p-pipes/pkg/common"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var ForwardingControlProtocolID = protocol.ID("/gop2ppipes/forward/control/0.1.0")
var ForwardingDataProtocolID = protocol.ID("/gop2ppipes/forward/data/0.1.0")

type SetupForwardingRequest struct {
	// TODO: we should obtain the peer id from rpc the request connection
	// not from rpc payload set by the peer
	ID      string
	Address string
}

type SetupForwardingResponse struct {
	AuthorizationCookie string
}

type ForwardingService struct {
	daemon *Daemon
}

func writeAuthorizationCookie(writer io.Writer, cookie string) error {
	var bytes = []byte(cookie)
	var size = uint64(len(bytes))
	var err error
	err = binary.Write(writer, binary.BigEndian, size)
	if err != nil {
		return err
	}
	var written uint64 = 0
	for written < size {
		n, err := writer.Write(bytes[written:])
		if err != nil {
			return err
		}
		written = written + uint64(n)
	}
	return nil
}

func readAuthorizationCookie(reader io.Reader) (string, error) {
	var size uint64
	if err := binary.Read(reader, binary.BigEndian, &size); err != nil {
		return "", err
	}
	b := make([]byte, size)
	if _, err := io.ReadFull(reader, b); err != nil {
		return "", err
	}
	return string(b), nil
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

	cookie, err := d.daemon.GetAuthorizationCookie(peer, addr)
	if err != nil {
		return err
	}
	response.AuthorizationCookie = cookie
	return nil
}

func (d *Daemon) RegisterForwardingService() error {
	rpcHost := gorpc.NewServer(d.Host, ForwardingControlProtocolID)
	svc := ForwardingService{d}
	if err := rpcHost.Register(&svc); err != nil {
		return err
	}
	streamHandler := func(s network.Stream) {
		daemon := d
		cookie, err := readAuthorizationCookie(s)
		if err != nil {
			log.Debugw("Failed to obtain authorization cookie", "error", err)
			s.Reset()
			return
		}
		addr, err := d.GetRemoteAddrFromAuthorizationCookie(cookie)
		if err != nil {
			s.Reset()
			log.Debugw("Resetted unauthorized stream", "id", s.ID(), "error", err)
			return
		}

		c, err := manet.Dial(addr)
		if err != nil {
			log.Debugw("Error dialing in server", "addr", addr.String(), "error", err)
			s.Reset()
			return
		}
		defer c.Close()
		defer s.Close()
		daemon.doStreamPipe(c, s)
	}

	d.SetStreamHandler(ForwardingDataProtocolID, streamHandler)
	return nil
}

// TODO: save a "real" cookie here and look up it later.
func (d *Daemon) GetAuthorizationCookie(peer peer.ID, addr multiaddr.Multiaddr) (string, error) {
	return peer.String() + "/" + addr.String(), nil
}

func (d *Daemon) GetRemoteAddrFromAuthorizationCookie(cookie string) (multiaddr.Multiaddr, error) {
	var ma multiaddr.Multiaddr
	addr := strings.SplitN(cookie, "/", 2)
	if len(addr) != 2 {
		return ma, errors.New("Invalid cookie")
	}
	return multiaddr.NewMultiaddr(addr[1])
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
	rpcClient := gorpc.NewClient(d.Host, ForwardingControlProtocolID)
	request := SetupForwardingRequest{
		ID:      d.ID().String(),
		Address: remoteAddr.String(),
	}
	var response SetupForwardingResponse
	err := rpcClient.Call(peer, "ForwardingService", "SetupForwarding", request, &response)
	if err != nil {
		log.Errorw("Error while calling rpc", "peer", peer, "error", err)
		return err
	}
	ctx := context.Background()
	s, err := d.NewStream(ctx, peer, ForwardingDataProtocolID)
	if err != nil {
		log.Errorw("Error while starting new stream to peer", "peer", peer, "error", err)
		return err
	}
	err = writeAuthorizationCookie(s, response.AuthorizationCookie)
	if err != nil {
		log.Errorw("Error while authorization cookie", "error", err)
		s.Reset()
		return err
	}
	listener, err := manet.Listen(localAddr)
	if err != nil {
		log.Errorw("Error while listening to local address", "addr", localAddr.String(), "error", err)
		s.Reset()
		return err
	}
	// Don't block waiting for local listener to accept connections.
	go func() {
		defer common.CloseMaNetListener(listener)
		c, err := listener.Accept()
		if err != nil {
			log.Debugw("Error while accepting connection to local address", "addr", localAddr.String(), "error", err)
			s.Reset()
			return
		}
		defer s.Close()
		defer c.Close()
		d.doStreamPipe(c, s)
	}()
	return nil
}
