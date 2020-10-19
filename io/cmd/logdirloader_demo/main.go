package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"

	"github.com/wchargin/tensorboard-data-server/fs"
	"github.com/wchargin/tensorboard-data-server/io/logdir"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

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

	readLogDirectory(args[0], len(*cpuprofile) == 0 && len(*memprofile) == 0)

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

func readLogDirectory(dir string, poll bool) {
	ll := logdir.LoaderBuilder{FS: fs.OS{}, Logdir: dir}.Start()
	stdin := bufio.NewReader(os.Stdin)
	for {
		c := make(chan struct{})
		go func() {
			ll.Reload()
			close(c)
		}()
		<-c

		runs := ll.Runs()
		runNames := make([]string, len(runs))
		{
			i := 0
			for k := range runs {
				runNames[i] = k
				i++
			}
		}
		sort.Strings(runNames)

		for _, run := range runNames {
			fmt.Printf("run %q\n", run)
			acc := runs[run]
			mds := acc.List()

			tags := make([]string, len(mds))
			{
				i := 0
				for k := range mds {
					tags[i] = k
					i++
				}
			}
			sort.Strings(tags)
			for _, tag := range tags {
				s := mds[tag].String()
				if len(s) > 60 {
					s = s[:57] + "..."
				}
				fmt.Printf("\ttag %q <%v>: meta=%v\n", tag, mds[tag].GetDataClass(), s)
				sample := acc.Sample(tag)
				if len(sample) > 0 {
					firstStep := sample[0].EventStep
					lastStep := sample[len(sample)-1].EventStep
					fmt.Printf("\t\tn=%d, firstStep=%d, lastStep=%d\n", len(sample), firstStep, lastStep)
				}
			}
		}

		if !poll {
			break
		}
		fmt.Printf("More (y/N)? ")
		line, err := stdin.ReadString('\n')
		if errors.Is(err, io.EOF) {
			fmt.Println()
			break
		}
		if err != nil {
			panic(err)
		}
		if line != "y\n" && line != "Y\n" {
			break
		}
	}
	if err := ll.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "closing logdir loader: %v\n", err)
		os.Exit(1)
	}
}
