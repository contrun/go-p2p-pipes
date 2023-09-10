package server

import (
	"context"
	"fmt"

	"github.com/contrun/go-p2p-pipes/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const UNIMPLEMENTED_ERROR_MESSAGE string = "unimplemented"

var UNIMPLEMENTED_ERROR error = fmt.Errorf("unimplemented")

func (s *Server) StartDiscoveringPeers(ctx context.Context, in *pb.StartDiscoveringPeersRequest) (*pb.StartDiscoveringPeersResponse, error) {
	var response pb.StartDiscoveringPeersResponse
	response.Result = pb.ResponseType(pb.ResponseType_ERROR.Number())
	response.Message = UNIMPLEMENTED_ERROR_MESSAGE
	return &response, UNIMPLEMENTED_ERROR
}

func (s *Server) StopDiscoveringPeers(ctx context.Context, in *pb.StopDiscoveringPeersRequest) (*pb.StopDiscoveringPeersResponse, error) {
	var response pb.StopDiscoveringPeersResponse
	response.Result = pb.ResponseType(pb.ResponseType_ERROR.Number())
	response.Message = UNIMPLEMENTED_ERROR_MESSAGE
	return &response, UNIMPLEMENTED_ERROR
}

func (s *Server) ListPeers(ctx context.Context, in *pb.ListPeersRequest) (*pb.ListPeersResponse, error) {
	var response pb.ListPeersResponse
	response.Result = pb.ResponseType(pb.ResponseType_ERROR.Number())
	response.Message = UNIMPLEMENTED_ERROR_MESSAGE
	return &response, UNIMPLEMENTED_ERROR
}

func (s *Server) ForwardIO(ctx context.Context, in *pb.ForwardIORequest) (*pb.ForwardIOResponse, error) {
	var response pb.ForwardIOResponse
	response.Result = pb.ResponseType(pb.ResponseType_ERROR.Number())
	response.Message = UNIMPLEMENTED_ERROR_MESSAGE
	return &response, UNIMPLEMENTED_ERROR
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
