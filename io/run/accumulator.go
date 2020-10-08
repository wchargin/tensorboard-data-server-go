package run

import (
	"fmt"
	"os"
	"sync"

	spb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/summary_go_proto"
	"github.com/wchargin/tensorboard-data-server/mem"
)

// NewAccumulator creates an Accumulator from a Reader's output channel and
// starts a goroutine to ingest data. The caller is still in charge of calling
// reader.Reload to wake up the reader.
func NewAccumulator(reader *Reader) *Accumulator {
	acc := &Accumulator{
		run:  reader.dir,
		c:    reader.Out,
		mds:  make(mem.MetadataStore),
		data: make(map[string]mem.EagerReservoir),
	}
	go acc.start()
	return acc
}

// An Accumulator maintains metadata and reservoir-sampled data for all time
// series within a single run.
type Accumulator struct {
	// run holds the name (filepath) of this run, for logging purposes.
	run string
	// c is the input channel for events, which is expected to be the
	// output channel of a run.Reader.
	c <-chan ValueResult

	// mu locks mds and data. mu is always held while data[_] accessed:
	// i.e., mu precedes the internal lock of data[_] in the total lock
	// ordering.
	mu sync.Mutex
	// mds holds the first SummaryMetadata for each seen tag.
	mds mem.MetadataStore
	// data maps from tag name to reservoir of values for that time series.
	data map[string]mem.EagerReservoir
}

// Step implements the StepIndexed interface.
func (d ValueDatum) Step() mem.Step {
	return d.EventStep
}

// start runs in its own goroutine, created by NewAccumulator.
func (acc *Accumulator) start() {
	for dr := range acc.c {
		if dr.Err != nil {
			// Swallow errors and print to console.
			fmt.Fprintln(os.Stderr, dr.Err)
			continue
		}
		datum := dr.Datum
		if datum == nil {
			fmt.Fprintf(os.Stderr, "run %q: empty ValueResult; aborting\n", acc.run)
			return
		}
		acc.ingestDatum(datum)
	}
}

// ingestDatum adds non-nil datum to the accumulator.
func (acc *Accumulator) ingestDatum(datum *ValueDatum) {
	if datum.Value == nil {
		fmt.Fprintf(os.Stderr, "run %q: skipping empty value at step %d\n", acc.run, datum.EventStep)
		return
	}
	tag := datum.Value.Tag

	acc.mu.Lock()
	defer acc.mu.Unlock()

	var md *spb.SummaryMetadata
	{
		var ok bool
		if md, ok = acc.mds[tag]; !ok {
			md = datum.Value.Metadata
			acc.mds[tag] = md
			if md == nil {
				fmt.Fprintf(os.Stderr, "run %q: skipping tag %q with no metadata\n", acc.run, tag)
			}
		}
	}
	if md == nil {
		return
	}
	rsv, ok := acc.data[tag]
	if !ok {
		rsv = mem.NewEagerReservoir(reservoirCapacity(md.DataClass))
		acc.data[tag] = rsv
	}
	rsv.Offer(*datum)
}

// List lists all tags with their summary metadata.
func (acc *Accumulator) List() mem.MetadataStore {
	result := make(mem.MetadataStore)
	acc.mu.Lock()
	defer acc.mu.Unlock()
	for k, v := range acc.mds {
		result[k] = v
	}
	return result
}

// Metadata returns the summary metadata for the given tag, or nil if it hasn't
// been seen. The summary metadata may also be nil if the tag has been seen but
// had nil metadata, in which case it will also have no data.
func (acc *Accumulator) Metadata(tag string) *spb.SummaryMetadata {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	return acc.mds[tag]
}

// Sample returns a sample of the values stored for the given time series, or
// an empty slice if no data has been seen. (It is not possible for a time
// series to be present with no data, so this is lossless.)
func (acc *Accumulator) Sample(tag string) []ValueDatum {
	acc.mu.Lock()
	rsv, ok := acc.data[tag]
	if !ok {
		acc.mu.Unlock()
		return nil
	}
	sample := rsv.Sample()
	acc.mu.Unlock()

	result := make([]ValueDatum, len(sample))
	for i, d := range sample {
		result[i] = d.(ValueDatum)
	}
	return result
}

// Last returns the last value stored for the given time series, or nil if no
// data has been seen.
func (acc *Accumulator) Last(tag string) *ValueDatum {
	acc.mu.Lock()
	rsv, ok := acc.data[tag]
	if !ok {
		acc.mu.Unlock()
		return nil
	}
	acc.mu.Unlock()
	last := rsv.Last()
	if last == nil {
		// shouldn't happen, but don't panic
		return nil
	}
	lastDatum := last.(ValueDatum)
	return &lastDatum
}

func reservoirCapacity(dc spb.DataClass) uint64 {
	switch dc {
	case spb.DataClass_DATA_CLASS_SCALAR:
		return 1000
	case spb.DataClass_DATA_CLASS_BLOB_SEQUENCE:
		return 10
	case spb.DataClass_DATA_CLASS_TENSOR:
		return 100
	default:
		return 10
	}
}
