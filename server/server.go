package server

import (
	"context"
	"encoding/binary"
	"log"
	"math"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/summary_go_proto"
	tpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/tensor_go_proto"
	dtpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/types_go_proto"
	"github.com/wchargin/tensorboard-data-server/io/logdir"
	"github.com/wchargin/tensorboard-data-server/io/run"
	"github.com/wchargin/tensorboard-data-server/mem"
	dppb "github.com/wchargin/tensorboard-data-server/proto/data_provider_proto"
)

// Server implements the TensorBoardDataProviderServer interface.
type Server struct {
	dppb.UnimplementedTensorBoardDataProviderServer
	ll *logdir.Loader
}

const (
	// blobBatchSizeBytes controls the pagination behavior of the ReadBlob
	// RPC. Only this many bytes will be sent per frame in the response
	// stream. Chosen as 8 MiB, which is reasonably small but exceeds the
	// default gRPC response size limit of 4 MiB. (Clients need to handle
	// larger responses from RPCs like ReadScalars, so this helps catch the
	// problem earlier.)
	blobBatchSizeBytes = 1024 * 1024 * 8
)

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
			sample := acc.Sample(tag)
			if len(sample) == 0 {
				// shouldn't happen, but don't panic
				continue
			}
			last := sample[len(sample)-1]
			e := &dppb.ListScalarsResponse_TagEntry{
				TagName: tag,
				TimeSeries: &dppb.ScalarTimeSeries{
					MaxStep:         int64(last.EventStep),
					MaxWallTime:     maxWallTime(sample),
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
				WallTime: make([]float64, len(sample)),
				Value:    make([]float64, len(sample)),
			}
			// TODO(@wchargin): Re-downsample.
			for i, x := range sample {
				data.Step[i] = int64(x.EventStep)
				data.WallTime[i] = x.EventWallTime
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

// ListTensors handles the ListTensors RPC.
func (s *Server) ListTensors(ctx context.Context, req *dppb.ListTensorsRequest) (*dppb.ListTensorsResponse, error) {
	res := new(dppb.ListTensorsResponse)
	runFilter, tagFilter := filters(req.RunTagFilter)

	for run, acc := range s.ll.Runs() {
		if !matchesFilter(runFilter, run) {
			continue
		}
		var tags []*dppb.ListTensorsResponse_TagEntry
		for tag, md := range acc.List() {
			if md == nil || md.DataClass != spb.DataClass_DATA_CLASS_TENSOR {
				continue
			}
			if md.PluginData.GetPluginName() != req.PluginFilter.GetPluginName() {
				continue
			}
			if !matchesFilter(tagFilter, tag) {
				continue
			}
			sample := acc.Sample(tag)
			if len(sample) == 0 {
				// shouldn't happen, but don't panic
				continue
			}
			last := sample[len(sample)-1]
			e := &dppb.ListTensorsResponse_TagEntry{
				TagName: tag,
				TimeSeries: &dppb.TensorTimeSeries{
					MaxStep:         int64(last.EventStep),
					MaxWallTime:     maxWallTime(sample),
					SummaryMetadata: md,
				},
			}
			tags = append(tags, e)
		}
		if tags != nil {
			e := &dppb.ListTensorsResponse_RunEntry{
				RunName: run,
				Tags:    tags,
			}
			res.Runs = append(res.Runs, e)
		}
	}
	return res, nil
}

// ReadTensors handles the ReadTensors RPC.
func (s *Server) ReadTensors(ctx context.Context, req *dppb.ReadTensorsRequest) (*dppb.ReadTensorsResponse, error) {
	res := new(dppb.ReadTensorsResponse)
	runFilter, tagFilter := filters(req.RunTagFilter)

	for run, acc := range s.ll.Runs() {
		if !matchesFilter(runFilter, run) {
			continue
		}
		var tags []*dppb.ReadTensorsResponse_TagEntry
		for tag, md := range acc.List() {
			if md == nil || md.DataClass != spb.DataClass_DATA_CLASS_TENSOR {
				continue
			}
			if md.PluginData.GetPluginName() != req.PluginFilter.GetPluginName() {
				continue
			}
			if !matchesFilter(tagFilter, tag) {
				continue
			}
			sample := acc.Sample(tag)
			data := dppb.TensorData{
				Step:     make([]int64, len(sample)),
				WallTime: make([]float64, len(sample)),
				Value:    make([]*tpb.TensorProto, len(sample)),
			}
			// TODO(@wchargin): Re-downsample.
			for i, x := range sample {
				data.Step[i] = int64(x.EventStep)
				data.WallTime[i] = x.EventWallTime
				data.Value[i] = x.Value.GetTensor()
			}
			e := &dppb.ReadTensorsResponse_TagEntry{
				TagName: tag,
				Data:    &data,
			}
			tags = append(tags, e)
		}
		if tags != nil {
			e := &dppb.ReadTensorsResponse_RunEntry{
				RunName: run,
				Tags:    tags,
			}
			res.Runs = append(res.Runs, e)
		}
	}
	return res, nil
}

// ListBlobSequences handles the ListBlobSequences RPC.
func (s *Server) ListBlobSequences(ctx context.Context, req *dppb.ListBlobSequencesRequest) (*dppb.ListBlobSequencesResponse, error) {
	res := new(dppb.ListBlobSequencesResponse)
	runFilter, tagFilter := filters(req.RunTagFilter)

	for run, acc := range s.ll.Runs() {
		if !matchesFilter(runFilter, run) {
			continue
		}
		var tags []*dppb.ListBlobSequencesResponse_TagEntry
		for tag, md := range acc.List() {
			if md == nil || md.DataClass != spb.DataClass_DATA_CLASS_BLOB_SEQUENCE {
				continue
			}
			if md.PluginData.GetPluginName() != req.PluginFilter.GetPluginName() {
				continue
			}
			if !matchesFilter(tagFilter, tag) {
				continue
			}
			sample := acc.Sample(tag)
			if len(sample) == 0 {
				// shouldn't happen, but don't panic
				continue
			}
			last := sample[len(sample)-1]
			e := &dppb.ListBlobSequencesResponse_TagEntry{
				TagName: tag,
				TimeSeries: &dppb.BlobSequenceTimeSeries{
					MaxStep:         int64(last.EventStep),
					MaxWallTime:     maxWallTime(sample),
					MaxLength:       maxLength(sample),
					SummaryMetadata: md,
				},
			}
			tags = append(tags, e)
		}
		if tags != nil {
			e := &dppb.ListBlobSequencesResponse_RunEntry{
				RunName: run,
				Tags:    tags,
			}
			res.Runs = append(res.Runs, e)
		}
	}
	return res, nil
}

// ReadBlobSequences handles the ReadBlobSequences RPC.
func (s *Server) ReadBlobSequences(ctx context.Context, req *dppb.ReadBlobSequencesRequest) (*dppb.ReadBlobSequencesResponse, error) {
	res := new(dppb.ReadBlobSequencesResponse)
	runFilter, tagFilter := filters(req.RunTagFilter)

	for run, acc := range s.ll.Runs() {
		if !matchesFilter(runFilter, run) {
			continue
		}
		var tags []*dppb.ReadBlobSequencesResponse_TagEntry
		for tag, md := range acc.List() {
			if md == nil || md.DataClass != spb.DataClass_DATA_CLASS_BLOB_SEQUENCE {
				continue
			}
			if md.PluginData.GetPluginName() != req.PluginFilter.GetPluginName() {
				continue
			}
			if !matchesFilter(tagFilter, tag) {
				continue
			}
			sample := acc.Sample(tag)
			data := dppb.BlobSequenceData{
				Step:     make([]int64, len(sample)),
				WallTime: make([]float64, len(sample)),
				Values:   make([]*dppb.BlobReferenceSequence, len(sample)),
			}
			// TODO(@wchargin): Re-downsample.
			for i, x := range sample {
				data.Step[i] = int64(x.EventStep)
				data.WallTime[i] = x.EventWallTime
				data.Values[i] = blobSequenceValues(req.ExperimentId, run, tag, x.EventStep, x.Value.GetTensor())
			}
			e := &dppb.ReadBlobSequencesResponse_TagEntry{
				TagName: tag,
				Data:    &data,
			}
			tags = append(tags, e)
		}
		if tags != nil {
			e := &dppb.ReadBlobSequencesResponse_RunEntry{
				RunName: run,
				Tags:    tags,
			}
			res.Runs = append(res.Runs, e)
		}
	}
	return res, nil
}

// ReadBlob handles the ReadBlob RPC.
func (s *Server) ReadBlob(req *dppb.ReadBlobRequest, stream dppb.TensorBoardDataProvider_ReadBlobServer) error {
	bk, err := decodeBlobKey(req.BlobKey)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid blob key %q: %v", req.BlobKey, err)
	}

	_ = bk.eid
	var data []run.ValueDatum
	if run := s.ll.Run(bk.run); run != nil {
		data = run.Sample(bk.tag)
	}
	if data == nil {
		return status.Errorf(codes.NotFound, "experiment %q has no time series for run %q, tag %q", bk.eid, bk.run, bk.tag)
	}

	var tensor *tpb.TensorProto
	for _, d := range data {
		if d.EventStep == bk.step {
			tensor = d.Value.GetTensor()
			break
		}
	}
	if tensor == nil {
		return status.Errorf(codes.NotFound, "time series for experiment %q, run %q, tag %q has no step %d; it may have been evicted from memory", bk.eid, bk.run, bk.tag, bk.step)
	}

	blobs := tensor.StringVal
	if bk.index >= int64(len(blobs)) {
		return status.Errorf(codes.NotFound, "time series for experiment %q, run %q, tag %q at step %d has no index %d (only %d items)", bk.eid, bk.run, bk.tag, bk.step, bk.index, len(blobs))
	}

	blob := blobs[bk.index]
	for len(blob) > blobBatchSizeBytes {
		frame := blob[:blobBatchSizeBytes]
		stream.Send(&dppb.ReadBlobResponse{Data: frame})
		blob = blob[blobBatchSizeBytes:]
	}
	if len(blob) > 0 {
		stream.Send(&dppb.ReadBlobResponse{Data: blob})
	}

	return nil
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

// scalarValue gets the scalar data point associated with the given tensor,
// whose summary's time series should be DATA_CLASS_SCALAR.
func blobSequenceValues(eid string, run string, tag string, step mem.Step, tensor *tpb.TensorProto) *dppb.BlobReferenceSequence {
	n := tensor.TensorShape.Dim[0].GetSize()
	refs := make([]*dppb.BlobReference, n)
	bk := blobKey{
		eid:  eid,
		run:  run,
		tag:  tag,
		step: step,
	}
	for i := int64(0); i < n; i++ {
		bk.index = i
		refs[i] = &dppb.BlobReference{BlobKey: string(bk.encode())}
	}
	return &dppb.BlobReferenceSequence{BlobRefs: refs}
}

func maxWallTime(ds []run.ValueDatum) float64 {
	result := math.Inf(-1)
	for _, d := range ds {
		wt := d.EventWallTime
		if wt > result {
			result = wt
		}
	}
	return result
}

// maxLength gets the maximum length of any of the given data points, whose
// summaries' time series should be DATA_CLASS_BLOB_SEQUENCE.
func maxLength(ds []run.ValueDatum) int64 {
	result := int64(-1)
	for _, d := range ds {
		length := d.Value.GetTensor().TensorShape.Dim[0].Size
		if length > result {
			result = length
		}
	}
	return result
}
