package server

import (
	"context"

	"github.com/contrun/go-p2p-pipes/pb"
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

		ch, err := s.Daemon.FindDHTPeersAsync(ctx, dht.String(), 0)
		if err != nil {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}

		go func() {
			for pi := range ch {
				log.Debugw("Found peer from DHT", "peerinfo", pi)
			}
		}()

		return &response, nil
	}

	return nil, UNIMPLEMENTED_ERROR
}

func (s *Server) StopDiscoveringPeers(ctx context.Context, in *pb.StopDiscoveringPeersRequest) (*pb.StopDiscoveringPeersResponse, error) {
	return nil, UNIMPLEMENTED_ERROR
}

func (s *Server) ListPeers(ctx context.Context, in *pb.ListPeersRequest) (*pb.ListPeersResponse, error) {
	return nil, UNIMPLEMENTED_ERROR
}

func (s *Server) ForwardIO(ctx context.Context, in *pb.ForwardIORequest) (*pb.ForwardIOResponse, error) {
	return nil, UNIMPLEMENTED_ERROR
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
