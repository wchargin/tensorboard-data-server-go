package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"

	"github.com/wchargin/tensorboard-data-server/fs"
	"github.com/wchargin/tensorboard-data-server/io/run"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

var bufsize = flag.Int("bufsize", 8192, "run.Reader.BufSize (bufio.NewReaderSize) capacity")
var chanbuf = flag.Int("chanbuf", 0, "make(chan ValueResult) capacity")

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "must specify run directory as only argument\n")
		os.Exit(1)
	}

	// Profiling structure copied from "go doc runtime/pprof".
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "creating CPU profile: %v\n", err)
			os.Exit(1)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	readRunDirectory(args[0])

	if *memprofile != "" {
		fmt.Println("creating memory profile...")
		f, err := os.Create(*memprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "creating memory profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "writing memory profile: %v\n", err)
		}
		fmt.Println("done with memory profile")
	}
}

func readRunDirectory(rundir string) {
	rr := run.ReaderBuilder{
		FS:      fs.OS{},
		Dir:     rundir,
		BufSize: *bufsize,
		ChanBuf: *chanbuf,
	}.Start()
	done := make(chan struct{})
	go func() {
		rr.Reload()
		close(done)
	}()
events:
	for {
		select {
		case res := <-rr.Out:
			if res.Err != nil {
				fmt.Fprintf(os.Stderr, "read error: %v\n", res)
			}
		case <-done:
			break events
		}
	}

	mds := rr.MetadataStore()
	tags := make([]string, len(mds))
	{
		i := 0
		for k := range mds {
			tags[i] = k
			i++
		}
		sort.Strings(tags)
		for _, tag := range tags {
			s := mds[tag].String()
			if len(s) > 60 {
				s = s[:57] + "..."
			}
			fmt.Printf("%v <%v>: meta=%v\n", tag, mds[tag].GetDataClass(), s)
		}
	}

	if err := rr.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "closing run reader: %v\n", err)
		os.Exit(1)
	}
}
