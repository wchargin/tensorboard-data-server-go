package mem

import (
	"testing"

	"github.com/golang/protobuf/proto"

	spb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/summary_go_proto"
	tpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/tensor_go_proto"
	tspb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/tensor_shape_go_proto"
	dtpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/types_go_proto"
	epb "github.com/tensorflow/tensorflow/tensorflow/go/core/util/event_go_proto"
)

func tfv1ScalarSummary(z float32) *epb.Event_Summary {
	return &epb.Event_Summary{Summary: &spb.Summary{
		Value: []*spb.Summary_Value{
			{Tag: "accuracy", Value: &spb.Summary_Value_SimpleValue{SimpleValue: z}},
		},
	}}
}

func tfv2ScalarSummary(content string, includeMeta bool) *epb.Event_Summary {
	tensor := &tpb.TensorProto{
		Dtype:         dtpb.DataType_DT_FLOAT,
		TensorShape:   &tspb.TensorShapeProto{},
		TensorContent: []byte(content),
	}
	value := &spb.Summary_Value{
		Tag:   "accuracy",
		Value: &spb.Summary_Value_Tensor{Tensor: tensor},
	}
	if includeMeta {
		value.Metadata = &spb.SummaryMetadata{
			// Note: No DataClass here. Legacy write paths omit it.
			PluginData: &spb.SummaryMetadata_PluginData{
				PluginName: scalarsPluginName,
			},
		}
	}
	return &epb.Event_Summary{Summary: &spb.Summary{Value: []*spb.Summary_Value{value}}}
}

func TestEventValuesTFv1Scalars(t *testing.T) {
	mds := make(MetadataStore)
	events := []*epb.Event{
		{Step: 0, WallTime: 1000.25, What: tfv1ScalarSummary(1.0)},
		{Step: 1, WallTime: 1234.50, What: tfv1ScalarSummary(7.0)},
	}
	var values []*spb.Summary_Value
	for _, e := range events {
		values = append(values, EventValues(e, mds)...)
	}

	wantScalars := []float32{1.0, 7.0}
	if got, want := len(values), len(wantScalars); got != want {
		t.Errorf("len(values): got %v, want %v: %v", got, want, values)
		if got < want {
			t.FailNow()
		}
	}

	// Check metadata.
	{
		got := values[0].Metadata
		want := &spb.SummaryMetadata{
			DataClass: spb.DataClass_DATA_CLASS_SCALAR,
			PluginData: &spb.SummaryMetadata_PluginData{
				PluginName: scalarsPluginName,
			},
		}
		if !proto.Equal(got, want) {
			t.Errorf("values[0].Metadata: got %v, want %v", got, want)
		}
		got = mds["accuracy"]
		if !proto.Equal(got, want) {
			t.Errorf(`mds["accuracy"]: got %v, want %v`, got, want)
		}
	}
	if got := values[1].Metadata; got != nil {
		t.Errorf("values[1].Metadata: got %v, want nil", got)
	}

	// Check values.
	for i, v := range values {
		tensor := v.GetTensor()
		if tensor == nil {
			t.Errorf("values[%v]: got %v, want Tensor", i, v)
		}
		wantTensor := &tpb.TensorProto{
			Dtype:       dtpb.DataType_DT_FLOAT,
			TensorShape: &tspb.TensorShapeProto{}, // rank-0 (scalar)
			FloatVal:    []float32{wantScalars[i]},
		}
		if !proto.Equal(tensor, wantTensor) {
			t.Errorf("values[%v].Tensor: got %v, want %v", i, tensor, wantTensor)
		}
	}
}

func TestEventValuesTFv2Scalars(t *testing.T) {
	mds := make(MetadataStore)
	events := []*epb.Event{
		{Step: 0, WallTime: 1000.25, What: tfv2ScalarSummary("\x00\x00\x80\x3f", true)},  // 1.0
		{Step: 1, WallTime: 1234.50, What: tfv2ScalarSummary("\x00\x00\xe0\x40", false)}, // 7.0
	}
	var values []*spb.Summary_Value
	for _, e := range events {
		values = append(values, EventValues(e, mds)...)
	}

	wantScalars := []string{"\x00\x00\x80\x3f", "\x00\x00\xe0\x40"}
	if got, want := len(values), len(wantScalars); got != want {
		t.Errorf("len(values): got %v, want %v: %v", got, want, values)
		if got < want {
			t.FailNow()
		}
	}

	// Check metadata.
	{
		got := values[0].Metadata
		want := &spb.SummaryMetadata{
			DataClass: spb.DataClass_DATA_CLASS_SCALAR,
			PluginData: &spb.SummaryMetadata_PluginData{
				PluginName: scalarsPluginName,
			},
		}
		if !proto.Equal(got, want) {
			t.Errorf("values[0].Metadata: got %v, want %v", got, want)
		}
		got = mds["accuracy"]
		if !proto.Equal(got, want) {
			t.Errorf(`mds["accuracy"]: got %v, want %v`, got, want)
		}
	}
	if got := values[1].Metadata; got != nil {
		t.Errorf("values[1].Metadata: got %v, want nil", got)
	}

	// Check values.
	for i, v := range values {
		tensor := v.GetTensor()
		if tensor == nil {
			t.Errorf("values[%v]: got %v, want Tensor", i, v)
		}
		wantTensor := &tpb.TensorProto{
			Dtype:         dtpb.DataType_DT_FLOAT,
			TensorShape:   &tspb.TensorShapeProto{}, // rank-0 (scalar)
			TensorContent: []byte(wantScalars[i]),
		}
		if !proto.Equal(tensor, wantTensor) {
			t.Errorf("values[%v].Tensor: got %v, want %v", i, tensor, wantTensor)
		}
	}
}

func TestEventValuesGraphDef(t *testing.T) {
	mds := make(MetadataStore)
	event := &epb.Event{
		Step:     0,
		WallTime: 1000.25,
		What:     &epb.Event_GraphDef{GraphDef: []byte("my graph")},
	}
	values := EventValues(event, mds)

	if got, want := len(values), 1; got != want {
		t.Errorf("len(values): got %v, want %v: %v", got, want, values)
		if got < want {
			t.FailNow()
		}
	}

	// Check metadata.
	{
		got := values[0].Metadata
		want := &spb.SummaryMetadata{
			DataClass: spb.DataClass_DATA_CLASS_BLOB_SEQUENCE,
			PluginData: &spb.SummaryMetadata_PluginData{
				PluginName: graphsPluginName,
			},
		}
		if !proto.Equal(got, want) {
			t.Errorf("values[0].Metadata: got %v, want %v", got, want)
		}
	}

	// Check values.
	{
		got := values[0].GetTensor()
		want := &tpb.TensorProto{
			Dtype:       dtpb.DataType_DT_STRING,
			TensorShape: &tspb.TensorShapeProto{Dim: []*tspb.TensorShapeProto_Dim{{Size: 1}}},
			StringVal:   [][]byte{[]byte("my graph")},
		}
		if !proto.Equal(got, want) {
			t.Errorf("values[0].Tensor: got %v, want %v", got, want)
		}
	}
}
