package server

import (
	"context"

	"github.com/wchargin/tensorboard-data-server/io/logdir"
	dppb "github.com/wchargin/tensorboard-data-server/proto/data_provider_proto"
)

// Server implements the TensorBoardDataProviderServer interface.
type Server struct {
	dppb.UnimplementedTensorBoardDataProviderServer
	ll *logdir.Loader
}

// NewServer creates an RPC server wrapper around a *logdir.Loader.
func NewServer(ll *logdir.Loader) *Server {
	return &Server{ll: ll}
}

// ListRuns handles the ListRuns RPC.
func (s *Server) ListRuns(ctx context.Context, req *dppb.ListRunsRequest) (*dppb.ListRunsResponse, error) {
	res := new(dppb.ListRunsResponse)
	runs := s.ll.Runs()
	res.Runs = make([]*dppb.Run, len(runs))
	{
		i := 0
		for run := range s.ll.Runs() {
			res.Runs[i] = &dppb.Run{Id: run, Name: run}
			i++
		}
	}
	return res, nil
}
