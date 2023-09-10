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
