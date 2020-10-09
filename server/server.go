package server

import (
	"context"
	"encoding/binary"
	"log"
	"math"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	spb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/summary_go_proto"
	tpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/tensor_go_proto"
	dtpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/types_go_proto"
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

// ListScalars handles the ListScalars RPC.
func (s *Server) ListScalars(ctx context.Context, req *dppb.ListScalarsRequest) (*dppb.ListScalarsResponse, error) {
	res := new(dppb.ListScalarsResponse)
	runFilter, tagFilter := filters(req.RunTagFilter)

	for run, acc := range s.ll.Runs() {
		if !matchesFilter(runFilter, run) {
			continue
		}
		var tags []*dppb.ListScalarsResponse_TagEntry
		for tag, md := range acc.List() {
			if md == nil || md.DataClass != spb.DataClass_DATA_CLASS_SCALAR {
				continue
			}
			if md.PluginData.GetPluginName() != req.PluginFilter.GetPluginName() {
				continue
			}
			if !matchesFilter(tagFilter, tag) {
				continue
			}
			last := acc.Last(tag)
			if last == nil {
				// shouldn't happen, but don't panic
				continue
			}
			e := &dppb.ListScalarsResponse_TagEntry{
				TagName: tag,
				TimeSeries: &dppb.ScalarTimeSeries{
					MaxStep:         int64(last.EventStep),
					MaxWallTime:     timestamp(last.EventWallTime),
					SummaryMetadata: md,
				},
			}
			tags = append(tags, e)
		}
		if tags != nil {
			e := &dppb.ListScalarsResponse_RunEntry{
				RunName: run,
				Tags:    tags,
			}
			res.Runs = append(res.Runs, e)
		}
	}
	return res, nil
}

// ReadScalars handles the ReadScalars RPC.
func (s *Server) ReadScalars(ctx context.Context, req *dppb.ReadScalarsRequest) (*dppb.ReadScalarsResponse, error) {
	res := new(dppb.ReadScalarsResponse)
	runFilter, tagFilter := filters(req.RunTagFilter)

	for run, acc := range s.ll.Runs() {
		if !matchesFilter(runFilter, run) {
			continue
		}
		var tags []*dppb.ReadScalarsResponse_TagEntry
		for tag, md := range acc.List() {
			if md == nil || md.DataClass != spb.DataClass_DATA_CLASS_SCALAR {
				continue
			}
			if md.PluginData.GetPluginName() != req.PluginFilter.GetPluginName() {
				continue
			}
			if !matchesFilter(tagFilter, tag) {
				continue
			}
			sample := acc.Sample(tag)
			data := dppb.ScalarData{
				Step:     make([]int64, len(sample)),
				WallTime: make([]*timestamppb.Timestamp, len(sample)),
				Value:    make([]float64, len(sample)),
			}
			// TODO(@wchargin): Re-downsample.
			for i, x := range sample {
				data.Step[i] = int64(x.EventStep)
				data.WallTime[i] = timestamp(x.EventWallTime)
				data.Value[i] = scalarValue(x.Value.GetTensor())
			}
			e := &dppb.ReadScalarsResponse_TagEntry{
				TagName: tag,
				Data:    &data,
			}
			tags = append(tags, e)
		}
		if tags != nil {
			e := &dppb.ReadScalarsResponse_RunEntry{
				RunName: run,
				Tags:    tags,
			}
			res.Runs = append(res.Runs, e)
		}
	}
	return res, nil
}

// A stringFilter is a predicate for strings. If nil, it matches all strings.
// Otherwise, it matches exactly those strings in the referenced slice.
type stringFilter *[]string

func matchesFilter(f stringFilter, x string) bool {
	if f == nil {
		return true
	}
	for _, y := range *f {
		if x == y {
			return true
		}
	}
	return false
}

// filters extracts two stringFilters from a *RunTagFilter, which may be nil.
// The returned filters may point into rtf.
func filters(rtf *dppb.RunTagFilter) (runs stringFilter, tags stringFilter) {
	if rf := rtf.GetRuns(); rf != nil {
		runs = &rf.Runs
	}
	if tf := rtf.GetTags(); tf != nil {
		tags = &tf.Tags
	}
	return
}

// timestamp converts event file wall_times to google.protobuf.Timestamp.
func timestamp(wallTime float64) *timestamppb.Timestamp {
	s, ns := math.Modf(wallTime) // OK if ns < 0
	t := time.Unix(int64(s), int64(ns*1e9))
	return timestamppb.New(t)
}

// scalarValue gets the scalar data point associated with the given tensor,
// whose summary's time series should be DATA_CLASS_SCALAR.
func scalarValue(tensor *tpb.TensorProto) float64 {
	switch tensor.Dtype {
	case dtpb.DataType_DT_FLOAT:
		if len(tensor.FloatVal) > 0 {
			return float64(tensor.FloatVal[0])
		}
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(tensor.TensorContent)))
	case dtpb.DataType_DT_DOUBLE:
		if len(tensor.DoubleVal) > 0 {
			return tensor.DoubleVal[0]
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(tensor.TensorContent))
	default:
		log.Printf("bad scalar dtype %v", tensor.Dtype)
		return math.NaN()
	}
}
