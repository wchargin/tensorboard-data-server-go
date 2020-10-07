package logdir

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/wchargin/tensorboard-data-server/fs"
	"github.com/wchargin/tensorboard-data-server/io/run"
)

type LoaderBuilder struct {
	// FS is the filesystem to use for read operations.
	FS fs.Filesystem
	// Logdir is the root log directory to be loaded, as a path under FS.
	Logdir string
}

func (b LoaderBuilder) Start() *Loader {
	ll := &Loader{
		fs:     b.FS,
		logdir: b.Logdir,

		readers: make(map[string]*run.Reader),
		data:    make(map[string]*run.Accumulator),

		reload: make(chan struct{}),
		asleep: make(chan struct{}),
	}
	go ll.start()
	return ll
}

type Loader struct {
	// fs is the filesystem to use for read operations.
	fs fs.Filesystem
	// logdir is the root log directory being loaded, as a path under fs.
	logdir string

	// reload is an input channel that sees unit when this loader should
	// wake up.
	reload chan struct{}
	// asleep is an output channel that sees unit when this loader has read
	// to EOF and gone to sleep, to be awoken later via "reload".
	asleep chan struct{}

	// mu locks the readers and data maps, not any of their contents.
	mu sync.RWMutex
	// readers maps a run name to its active reader object.
	readers map[string]*run.Reader
	// data maps a run name to its event accumulator. Same domain as
	// readers.
	data map[string]*run.Accumulator
}

// Runs returns a map of all runs, keyed by name. The returned map is owned by
// the caller; its values are not.
func (ll *Loader) Runs() map[string]*run.Accumulator {
	ll.mu.RLock()
	defer ll.mu.RUnlock()
	result := make(map[string]*run.Accumulator)
	for k, v := range ll.data {
		result[k] = v
	}
	return result
}

// Run returns the accumulator for the run with the given name (not path), or
// nil if there is no such run.
func (ll *Loader) Run(run string) *run.Accumulator {
	ll.mu.RLock()
	defer ll.mu.RUnlock()
	return ll.data[run]
}

// start runs in its own goroutine, created by LoaderBuilder.Start.
func (ll *Loader) start() {
	for range ll.reload {
		rundirs, err := ll.rundirs()
		if err != nil {
			fmt.Fprintf(os.Stderr, "discovering runs: %v\n", err)
			continue
		}
		ll.mkloaders(rundirs)
		ll.doreload()
		ll.asleep <- struct{}{}
	}
}

// A rundirs value maps a run name to its path.
type rundirs map[string]string

// rundirs finds all run directories under the logdir, by looking for all
// tfevents files.
func (ll *Loader) rundirs() (rundirs, error) {
	result := make(rundirs)
	files, err := ll.fs.FindFiles(ll.logdir, "*tfevents*")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		d := filepath.Dir(f) // TODO(@wchargin): fs integration?
		if _, ok := result[d]; ok {
			continue
		}
		name, err := filepath.Rel(ll.logdir, d)
		if err != nil {
			return nil, err
		}
		result[name] = d
	}
	return result, nil
}

// mkloaders synchronizes ll.readers and ll.data with the provided rundirs.
func (ll *Loader) mkloaders(rundirs rundirs) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	for k, rr := range ll.readers {
		if _, ok := rundirs[k]; ok {
			continue // still exists
		}
		fmt.Fprintf(os.Stderr, "removing run %q\n", k)
		if err := rr.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing run %q: %v\n", k, err)
		}
		delete(ll.readers, k)
		delete(ll.data, k)
	}
	for k, dir := range rundirs {
		if _, ok := ll.readers[k]; ok {
			continue // already exists
		}
		fmt.Fprintf(os.Stderr, "discovered run %q\n", k)
		rr := run.ReaderBuilder{FS: ll.fs, Dir: dir}.Start()
		acc := run.NewAccumulator(rr)
		ll.readers[k] = rr
		ll.data[k] = acc
	}
}

func (ll *Loader) doreload() {
	var wg sync.WaitGroup

	ll.mu.RLock()
	readers := make([]*run.Reader, len(ll.readers))
	{
		i := 0
		for _, rr := range ll.readers {
			readers[i] = rr
			i++
		}
	}
	ll.mu.RUnlock()

	wg.Add(len(readers))
	for _, rr := range readers {
		go func(rr *run.Reader) {
			rr.Reload()
			wg.Done()
		}(rr)
	}
	wg.Wait()
}

// Reload polls the log directory and reloads runs. It blocks until the reload
// finishes. Must not be called concurrently with any other Reload. May be
// called concurrently with reads.
func (ll *Loader) Reload() {
	ll.reload <- struct{}{}
	<-ll.asleep
}

// Close implements io.Closer. If there are multiple errors when closing
// underlying run readers, an arbitrary one is returned.
func (ll *Loader) Close() error {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	ll.data = nil // gc
	var firstErr error
	for _, rr := range ll.readers {
		err := rr.Close()
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
