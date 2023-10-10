package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"github.com/contrun/go-p2p-pipes/pb"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const UNIMPLEMENTED_ERROR_MESSAGE string = "unimplemented"

var UNIMPLEMENTED_ERROR error = status.Error(codes.Unimplemented, UNIMPLEMENTED_ERROR_MESSAGE)

func (s *Server) StartDiscoveringPeers(ctx context.Context, in *pb.StartDiscoveringPeersRequest) (*pb.StartDiscoveringPeersResponse, error) {
	var response pb.StartDiscoveringPeersResponse
	if in.Method == pb.PeerDiscoveryMethod_DHT {
		dht := in.GetDht()
		if dht == nil {
			return nil, status.Error(codes.InvalidArgument, "Argument dht not passed")
		}
		rv := dht.GetRv()
		if rv == "" {
			return nil, status.Error(codes.InvalidArgument, "Argument rv dht not passed")
		}

		err := s.Daemon.AddDHTRendezvous(ctx, rv)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		return &response, nil
	}

	return nil, status.Error(codes.InvalidArgument, "Unsupported method")
}

func (s *Server) StopDiscoveringPeers(ctx context.Context, in *pb.StopDiscoveringPeersRequest) (*pb.StopDiscoveringPeersResponse, error) {
	var response pb.StopDiscoveringPeersResponse
	if in.Method == pb.PeerDiscoveryMethod_DHT {
		dht := in.GetDht()
		if dht == nil {
			return nil, status.Error(codes.InvalidArgument, "Argument dht not passed")
		}
		rv := dht.GetRv()
		if rv == "" {
			return nil, status.Error(codes.InvalidArgument, "Argument rv dht not passed")
		}

		err := s.Daemon.DeleteDHTRendezvous(ctx, rv)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		return &response, nil
	}

	return nil, status.Error(codes.InvalidArgument, "Unsupported method")
}

func (s *Server) ListPeers(ctx context.Context, in *pb.ListPeersRequest) (*pb.ListPeersResponse, error) {
	var response pb.ListPeersResponse
	switch in.GetPeerType() {
	case pb.PeerType_PEERTYPEUNDEFIEND, pb.PeerType_ALL, pb.PeerType_CONNECTED:
		n := s.Daemon.Network()
		ps := n.Peerstore()
		peers := ps.Peers()
		peerinfos := make([]peer.AddrInfo, len(peers))
		for _, p := range peers {
			addrinfo := ps.PeerInfo(p)
			if in.GetPeerType() == pb.PeerType_CONNECTED && n.Connectedness(p) != network.Connected {
				continue
			}
			peerinfos = append(peerinfos, addrinfo)
		}
		response.Peers = addrInfosToPBPeers(n, peerinfos)
		return &response, nil
	default:
		return nil, UNIMPLEMENTED_ERROR
	}
}

func (s *Server) ListDiscoveredPeers(ctx context.Context, in *pb.ListDiscoveredPeersRequest) (*pb.ListDiscoveredPeersResponse, error) {
	var response pb.ListDiscoveredPeersResponse
	if in.Method == pb.PeerDiscoveryMethod_DHT {
		dht := in.GetDht()
		if dht == nil {
			return nil, status.Error(codes.InvalidArgument, "Argument dht not passed")
		}

		rv := dht.GetRv()
		if rv == "" {
			return nil, status.Error(codes.InvalidArgument, "Argument rv dht not passed")
		}

		peers, err := s.Daemon.FindPeersViaDHTSync(ctx, rv)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		log.Debugw("Found peers via DHT", "#peers", len(peers), "peers", peers)
		response.Peers = addrInfosToPBPeers(s.Daemon.Network(), peers)

		return &response, nil
	}

	return nil, UNIMPLEMENTED_ERROR
}

func addrInfoToPBPeer(network network.Network, peer peer.AddrInfo) *pb.Peer {
	var p pb.Peer
	p.Id = peer.ID.String()
	for _, addr := range peer.Addrs {
		p.Addresses = append(p.Addresses, addr.String())
		if network != nil {
			p.Connectedness = network.Connectedness(peer.ID).String()
			conns := network.ConnsToPeer(peer.ID)
			for _, conn := range conns {
				var c pb.Connection
				c.Id = conn.ID()
				c.Direction = conn.Stat().Direction.String()
				c.IsTransient = conn.Stat().Transient
				c.OpenTime = conn.Stat().Opened.String()
				c.LocalAddr = conn.LocalMultiaddr().String()
				c.RemoteAddr = conn.RemoteMultiaddr().String()
				pk, err := conn.RemotePublicKey().Raw()
				if err != nil {
					c.RemotePublicKey = base64.StdEncoding.EncodeToString(pk)
				}
				c.Multiplexer = string(conn.ConnState().StreamMultiplexer)
				c.Security = string(conn.ConnState().Security)
				c.Transport = conn.ConnState().Transport
				for _, stream := range conn.GetStreams() {
					var s pb.Stream
					s.ConnectionId = stream.Conn().ID()
					s.Id = stream.ID()
					s.Protocol = string(stream.Protocol())
					c.Streams = append(c.Streams, &s)
				}
				p.Connections = append(p.Connections, &c)
			}
		}
	}
	return &p
}

func addrInfosToPBPeers(network network.Network, peers []peer.AddrInfo) []*pb.Peer {
	var ps = make([]*pb.Peer, len(peers))
	for _, peer := range peers {
		ps = append(ps, addrInfoToPBPeer(network, peer))
	}
	return ps
}

func validateIO(io *pb.IO) error {
	if io == nil {
		return fmt.Errorf("IO should not be nil")
	}
	switch io.IoType {
	case pb.IOType_IOTYPEUNDEFINED:
		return fmt.Errorf("IO type is undefined")
	case pb.IOType_TCP:
		if io.GetTcp() == "" {
			return fmt.Errorf("Address not provided in Tcp")
		} else {
			return nil
		}
	case pb.IOType_UDP:
		if io.GetUdp() == "" {
			return fmt.Errorf("Address not provided in Udp")
		} else {
			return nil
		}
	case pb.IOType_UNIX:
		if io.GetUnix() == "" {
			return fmt.Errorf("Address not provided in Unix")
		} else {
			return nil
		}
	default:
		return nil
	}
}

func ioToMultiaddr(io *pb.IO) (multiaddr.Multiaddr, error) {
	var ma multiaddr.Multiaddr
	if err := validateIO(io); err != nil {
		return ma, err
	}

	switch io.IoType {
	case pb.IOType_TCP:
		addr, err := net.ResolveTCPAddr("tcp", io.GetTcp())
		if err != nil {
			return ma, err
		}
		return manet.FromNetAddr(addr)
	case pb.IOType_UDP:
		addr, err := net.ResolveUDPAddr("udp", io.GetUdp())
		if err != nil {
			return ma, err
		}
		return manet.FromNetAddr(addr)
	case pb.IOType_UNIX:
		addr, err := net.ResolveUnixAddr("unix", io.GetUnix())
		if err != nil {
			return ma, err
		}
		return manet.FromNetAddr(addr)
	default:
		return ma, fmt.Errorf("Unsupported IO type %s", io.IoType)
	}
}

func (s *Server) StartForwardingIO(ctx context.Context, in *pb.StartForwardingIORequest) (*pb.StartForwardingIOResponse, error) {
	var response pb.StartForwardingIOResponse
	if in.Peer == nil {
		return nil, status.Error(codes.InvalidArgument, "Peer not given")
	}
	peer, err := peer.Decode(in.Peer.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid peer id %s", in.Peer.GetId()))
	}
	if addrs := in.Peer.GetAddresses(); len(addrs) != 0 {
		mas := make([]multiaddr.Multiaddr, len(addrs))
		for _, addr := range addrs {
			ma, err := multiaddr.NewMultiaddr(addr)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid peer addr %s", addr))
			}
			mas = append(mas, ma)
		}
		s.Daemon.Peerstore().AddAddrs(peer, mas, time.Duration(time.Second*60))
	}
	if in.RemoteIo == nil || in.LocalIo == nil {
		return nil, status.Error(codes.InvalidArgument, "Addresses to forward traffic not given")
	}
	remoteAddr, err := ioToMultiaddr(in.RemoteIo)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid remote addr %s", in.RemoteIo))
	}
	localAddr, err := ioToMultiaddr(in.LocalIo)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid local addr %s", in.LocalIo))
	}
	err = s.Daemon.ForwardTraffic(peer, remoteAddr, localAddr)
	if err != nil {
		return &response, status.Error(codes.Internal, err.Error())
	}
	return &response, nil
}

func (server *Server) listen() {
	s := grpc.NewServer()
	pb.RegisterP2PPipeServer(s, server)
	reflection.Register(s)
	if err := s.Serve(server.Listener); err != nil {
		server.Close()
		log.Fatalf("failed to serve: %v", err)
	}
}
