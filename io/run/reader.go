package run

import (
	"bufio"
	"io"
	"sort"
	"strings"

	spb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/summary_go_proto"
	epb "github.com/tensorflow/tensorflow/tensorflow/go/core/util/event_go_proto"
	"github.com/wchargin/tensorboard-data-server/fs"
	"github.com/wchargin/tensorboard-data-server/io/eventfile"
	"github.com/wchargin/tensorboard-data-server/mem"
)

// eventFileInfix appears in a file name if and only if that file is an event
// file.
const eventFileInfix = "tfevents"

// A ValueDatum holds a Summary.Value protobuf with the step and wall time from
// the enclosing event.
type ValueDatum struct {
	Step     mem.Step
	WallTime float64
	Value    *spb.Summary_Value
}

// A ValueResult is either a non-nil Datum plus a nil Err or a nil Datum plus a
// non-nil Err.
type ValueResult struct {
	Datum *ValueDatum
	Err   error
}

// ReaderBuilder specifies options for a Reader.
type ReaderBuilder struct {
	// FS is the filesystem to use for read operations.
	FS fs.Filesystem
	// Dir is the directory being loaded, as a path under FS.
	Dir string

	// BufSize controls the size of the reader buffer on the underlying
	// event file. It's optional; zero means to use a default.
	BufSize int
}

// Reader reads events from all event files in a directory and streams their
// values after compatibility transformations. Call Reload while listening to
// its Out channel.
type Reader struct {
	// Out is the output channel for values and errors.
	Out <-chan ValueResult
	readerState
}

// Reader holds the internal state for a loading goroutine. All fields
// are owned by the goroutine and not intended for multithreaded access.
type readerState struct {
	// fs is the filesystem to use for read operations.
	fs fs.Filesystem
	// dir is the directory being loaded, as a path under fs.
	dir string
	// loaders is a map of stateful readers for open event files, or nil if
	// a reader has been closed due to a fatal error.
	loaders map[string]*eventfile.Reader
	// fds is a map of underlying file descriptors that need to be closed.
	fds map[string]fs.File
	// newBufioReader is a *bufio.Reader factory, either NewReader or a
	// partially applied NewReaderSize.
	newBufioReader func(io.Reader) *bufio.Reader

	// mds holds the first SummaryMetadata for each seen tag.
	mds mem.MetadataStore

	// Communication channels, described from the perspective of the
	// loading goroutine (*Reader.start).

	// reload is an input channel that sees "true" when this loader should
	// wake up.
	reload chan struct{}
	// asleep is an output channel that sees "true" when this loader has
	// read to EOF and gone to sleep, to be awoken later via "reload".
	asleep chan struct{}
	// out is the output channel along which results will be sent.
	out chan<- ValueResult
}

// Start starts a reader in a new goroutine. Once woken with a call to Reload,
// it reads the full contents of the run directory, then goes to sleep again.
func (b ReaderBuilder) Start() *Reader {
	newBufioReader := bufio.NewReader
	bufSize := b.BufSize
	if bufSize != 0 {
		newBufioReader = func(r io.Reader) *bufio.Reader { return bufio.NewReaderSize(r, bufSize) }
	}
	out := make(chan ValueResult)
	st := readerState{
		fs:             b.FS,
		dir:            b.Dir,
		loaders:        make(map[string]*eventfile.Reader),
		fds:            make(map[string]fs.File),
		newBufioReader: newBufioReader,

		mds: make(map[string]*spb.SummaryMetadata),
		out: out,

		reload: make(chan struct{}),
		asleep: make(chan struct{}),
	}
	rr := &Reader{Out: out, readerState: st}
	go rr.start()
	return rr
}

func (rr *Reader) start() {
	for range rr.reload {
		rr.mkloaders()
		filenames := make([]string, len(rr.loaders))
		{
			i := 0
			for k := range rr.loaders {
				filenames[i] = k
				i++
			}
		}
		sort.Strings(filenames)
		for _, fn := range filenames {
			rr.readfrom(fn)
		}
		rr.asleep <- struct{}{}
	}
}

// mkloaders ensures that a loader exists for every event file in the run
// directory. It sends zero or more error results along rr.out.
func (rr *Reader) mkloaders() {
	files, err := rr.fs.ListFiles(rr.dir)
	if err != nil {
		rr.out <- ValueResult{Err: err}
		return
	}
	for _, file := range files {
		if !strings.Contains(file, eventFileInfix) {
			continue
		}
		err := rr.mkloader(file)
		if err != nil {
			rr.out <- ValueResult{Err: err}
		}
	}
}

func (rr *Reader) mkloader(file string) error {
	if _, ok := rr.loaders[file]; ok {
		return nil
	}
	fd, err := rr.fs.Open(file)
	if err != nil {
		return err
	}
	rr.fds[file] = fd
	br := rr.newBufioReader(fd)
	er := eventfile.ReaderBuilder{File: br}.Start()
	rr.loaders[file] = er
	return nil
}

func (rr *Reader) readfrom(file string) {
	efr := rr.loaders[file]
	if efr == nil {
		return // loader already aborted
	}
	efr.Wake <- eventfile.Resume
	for {
		select {
		case <-efr.Asleep:
			return
		case res := <-efr.Results:
			if res.Fatal {
				rr.loaders[file] = nil
				rr.out <- ValueResult{Err: res.Err}
				// Defer closing rr.fds[file] until rr.Close().
				break
			}
			if res.Err != nil {
				rr.out <- ValueResult{Err: res.Err}
				continue
			}
			rr.sendValues(res.Event)
		}
	}
}

func (rr *Reader) sendValues(ev *epb.Event) {
	for _, v := range mem.EventValues(ev, rr.mds) {
		datum := &ValueDatum{
			Step:     mem.Step(ev.Step),
			WallTime: ev.WallTime,
			Value:    v,
		}
		rr.out <- ValueResult{Datum: datum}
	}
}

// Close implements io.Closer. If there are multiple errors when closing
// underlying files, an arbitrary one is returned.
func (rr *Reader) Close() error {
	for _, efr := range rr.loaders {
		if efr == nil {
			continue
		}
		go func(efr *eventfile.Reader) {
			efr.Wake <- eventfile.Abort
		}(efr)
	}
	rr.loaders = nil // gc
	var firstErr error
	for _, f := range rr.fds {
		if f == nil {
			continue
		}
		err := f.Close()
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Reload polls event files again and reads them to current EOF. It blocks
// until the reload finishes. Must not be called concurrently with any method
// on the reader, including another call to Reload.
func (rr *Reader) Reload() {
	rr.reload <- struct{}{}
	<-rr.asleep
}

// MetadataStore returns the mem.MetadataStore tracked by this reader, which
// must not be accessed concurrent with any Reload.
func (rr *Reader) MetadataStore() mem.MetadataStore {
	return rr.mds
}
