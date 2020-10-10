package server

import (
	"testing"

	tpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/tensor_go_proto"
	tspb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/tensor_shape_go_proto"
	dtpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/types_go_proto"
)

func TestScalarValueFloatVal(t *testing.T) {
	tensor := &tpb.TensorProto{
		TensorShape: &tspb.TensorShapeProto{Dim: nil},
		Dtype:       dtpb.DataType_DT_FLOAT,
		FloatVal:    []float32{777.0},
	}
	if got, want := scalarValue(tensor), 777.0; got != want {
		t.Errorf("scalarValue(%v): got %v, want %v", tensor, got, want)
	}
}

func TestScalarValueFloatBytes(t *testing.T) {
	tensor := &tpb.TensorProto{
		TensorShape:   &tspb.TensorShapeProto{Dim: nil},
		Dtype:         dtpb.DataType_DT_FLOAT,
		TensorContent: []byte("\x00\x40\x42\x44"),
	}
	if got, want := scalarValue(tensor), 777.0; got != want {
		t.Errorf("scalarValue(%v): got %v, want %v", tensor, got, want)
	}
}

func TestScalarValueDoubleVal(t *testing.T) {
	tensor := &tpb.TensorProto{
		TensorShape: &tspb.TensorShapeProto{Dim: nil},
		Dtype:       dtpb.DataType_DT_DOUBLE,
		DoubleVal:   []float64{1554.0},
	}
	if got, want := scalarValue(tensor), 1554.0; got != want {
		t.Errorf("scalarValue(%v): got %v, want %v", tensor, got, want)
	}
}

func TestScalarValueDoubleBytes(t *testing.T) {
	tensor := &tpb.TensorProto{
		TensorShape:   &tspb.TensorShapeProto{Dim: nil},
		Dtype:         dtpb.DataType_DT_DOUBLE,
		TensorContent: []byte("\x00\x00\x00\x00\x00\x90\x96\x40"),
	}
	if got, want := scalarValue(tensor), 1444.0; got != want {
		t.Errorf("scalarValue(%v): got %v, want %v", tensor, got, want)
	}
}
