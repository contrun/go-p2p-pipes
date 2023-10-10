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
	if dht := in.GetDht(); dht != nil {
		var response pb.StartDiscoveringPeersResponse

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
	if dht := in.GetDht(); dht != nil {
		var response pb.StopDiscoveringPeersResponse

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
		peerinfos := make([]peer.AddrInfo, 0)
		for _, p := range peers {
			addrinfo := ps.PeerInfo(p)
			if s.Daemon.Host.ID() == p {
				continue
			}
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
	if dht := in.GetDht(); dht != nil {
		var response pb.ListDiscoveredPeersResponse

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
	var ps = make([]*pb.Peer, 0)
	for _, peer := range peers {
		ps = append(ps, addrInfoToPBPeer(network, peer))
	}
	return ps
}

func ioToMultiaddr(io *pb.IO) (multiaddr.Multiaddr, error) {
	var ma multiaddr.Multiaddr

	if tcp := io.GetTcp(); tcp != "" {
		addr, err := net.ResolveTCPAddr("tcp", tcp)
		if err != nil {
			return ma, err
		}
		return manet.FromNetAddr(addr)
	}
	if udp := io.GetUdp(); udp != "" {
		addr, err := net.ResolveUDPAddr("udp", udp)
		if err != nil {
			return ma, err
		}
		return manet.FromNetAddr(addr)
	}
	if unix := io.GetUnix(); unix != "" {
		addr, err := net.ResolveUnixAddr("unix", unix)
		if err != nil {
			return ma, err
		}
		return manet.FromNetAddr(addr)
	}
	return ma, fmt.Errorf("Unsupported IO type")
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
	if in.Remote == nil || in.Local == nil {
		return nil, status.Error(codes.InvalidArgument, "Addresses to forward traffic not given")
	}
	remoteAddr, err := ioToMultiaddr(in.Remote)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid remote addr %s", in.Remote))
	}
	localAddr, err := ioToMultiaddr(in.Local)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Invalid local addr %s", in.Local))
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
