package mem

import (
	"fmt"

	spb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/summary_go_proto"
	tpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/tensor_go_proto"
	tspb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/tensor_shape_go_proto"
	dtpb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/types_go_proto"
	epb "github.com/tensorflow/tensorflow/tensorflow/go/core/util/event_go_proto"
)

// A MetadataStore holds the initial SummaryMetadata seen for each tag within a
// single run. Keys are tag names; the run is implicit.
type MetadataStore map[string]*spb.SummaryMetadata

// runGraphName is the tag name used for run-level `graph_def`s. Must agree
// with TensorBoard's `tensorboard.plugins.graph.metadata.RUN_GRAPH_NAME`.
const runGraphName string = "__run_graph__"

// TensorBoard plugin names; must agree with the `PLUGIN_NAME`s defined in
// `tensorboard.plugin.*.metadata`.
const (
	graphsPluginName  = "graphs"
	imagesPluginName  = "images"
	scalarsPluginName = "scalars"
)

// EventValues converts an on-disk event to the summary values that it
// represents, applying compatibility transformations. It updates the
// MetadataStore with any new summary metadata, and may read from it to
// determine how to transform summary data. The input event may be mutated.
func EventValues(e *epb.Event, mds MetadataStore) []*spb.Summary_Value {
	switch what := e.What.(type) {
	case *epb.Event_GraphDef:
		return migrateGraphDef(what, mds)
	case *epb.Event_Summary:
		return migrateSummary(what, mds)
	}
	return nil
}

func migrateGraphDef(gd *epb.Event_GraphDef, mds MetadataStore) []*spb.Summary_Value {
	tensor := &tpb.TensorProto{
		Dtype:       dtpb.DataType_DT_STRING,
		TensorShape: &tspb.TensorShapeProto{Dim: []*tspb.TensorShapeProto_Dim{{Size: 1}}},
		StringVal:   [][]byte{gd.GraphDef},
	}
	v := &spb.Summary_Value{
		Tag:   runGraphName,
		Value: &spb.Summary_Value_Tensor{Tensor: tensor},
	}
	if _, hasMeta := mds[runGraphName]; !hasMeta {
		v.Metadata = &spb.SummaryMetadata{
			PluginData: &spb.SummaryMetadata_PluginData{
				PluginName: graphsPluginName,
			},
			DataClass: spb.DataClass_DATA_CLASS_BLOB_SEQUENCE,
		}
		mds[runGraphName] = v.Metadata
	}
	return []*spb.Summary_Value{v}
}

func migrateSummary(s *epb.Event_Summary, mds MetadataStore) []*spb.Summary_Value {
	var result []*spb.Summary_Value
	for _, v := range s.Summary.Value {
		meta, hadMeta := mds[v.Tag]
		migrateValueInPlace(v, meta)
		if !hadMeta {
			mds[v.Tag] = v.Metadata
		}
		result = append(result, v)
	}
	return result
}

func migrateValueInPlace(v *spb.Summary_Value, initialMeta *spb.SummaryMetadata) {
	switch what := v.Value.(type) {
	case *spb.Summary_Value_SimpleValue:
		tensor := &tpb.TensorProto{
			Dtype:       dtpb.DataType_DT_FLOAT,
			TensorShape: &tspb.TensorShapeProto{},
			FloatVal:    []float32{what.SimpleValue},
		}
		v.Value = &spb.Summary_Value_Tensor{Tensor: tensor}
		if initialMeta == nil {
			v.Metadata = &spb.SummaryMetadata{
				PluginData: &spb.SummaryMetadata_PluginData{
					PluginName: scalarsPluginName,
				},
				DataClass: spb.DataClass_DATA_CLASS_SCALAR,
			}
		}
	case *spb.Summary_Value_Image:
		im := what.Image
		bufs := [][]byte{
			[]byte(fmt.Sprintf("%d", im.Width)),
			[]byte(fmt.Sprintf("%d", im.Height)),
			im.EncodedImageString,
		}
		tensor := &tpb.TensorProto{
			Dtype:       dtpb.DataType_DT_STRING,
			TensorShape: &tspb.TensorShapeProto{Dim: []*tspb.TensorShapeProto_Dim{{Size: 3}}},
			StringVal:   bufs,
		}
		v.Value = &spb.Summary_Value_Tensor{Tensor: tensor}
		if initialMeta == nil {
			v.Metadata = &spb.SummaryMetadata{
				PluginData: &spb.SummaryMetadata_PluginData{
					PluginName: imagesPluginName,
				},
				DataClass: spb.DataClass_DATA_CLASS_BLOB_SEQUENCE,
			}
		}
	case *spb.Summary_Value_Tensor:
		migrateTensorInPlace(v, what.Tensor, initialMeta)
	default:
		// Ignore other values for now.
	}
}

func migrateTensorInPlace(v *spb.Summary_Value, tensor *tpb.TensorProto, initialMeta *spb.SummaryMetadata) {
	var pluginName string
	if initialMeta == nil {
		pluginName = v.Metadata.GetPluginData().GetPluginName()
		switch pluginName {
		case scalarsPluginName:
			v.Metadata.DataClass = spb.DataClass_DATA_CLASS_SCALAR
		case imagesPluginName:
			v.Metadata.DataClass = spb.DataClass_DATA_CLASS_BLOB_SEQUENCE
		}
	}
	switch pluginName {
	case scalarsPluginName:
		// no transformations needed
	case imagesPluginName:
		// no transformations needed
	}
}
