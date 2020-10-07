package main

import (
	"context"
	"log"
	"net"

	dppb "github.com/wchargin/tensorboard-data-server/proto/data_provider_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	port = ":6106"
)

type server struct {
	dppb.UnimplementedTensorBoardDataProviderServer
}

func (*server) ListRuns(ctx context.Context, req *dppb.ListRunsRequest) (*dppb.ListRunsResponse, error) {
	res := new(dppb.ListRunsResponse)
	res.Runs = []*dppb.Run{
		&dppb.Run{Name: "hello"},
		&dppb.Run{Name: "tensorboard"},
	}
	return res, nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	dppb.RegisterTensorBoardDataProviderServer(s, &server{})
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
