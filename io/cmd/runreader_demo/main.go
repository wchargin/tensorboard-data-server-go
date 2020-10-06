package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/wchargin/tensorboard-data-server/fs"
	tbio "github.com/wchargin/tensorboard-data-server/io"
	"github.com/wchargin/tensorboard-data-server/io/run"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "must specify run directory as first argument\n")
		os.Exit(1)
	}
	rundir := os.Args[1]
	rr := run.ReaderBuilder{FS: fs.OS{}, Dir: rundir}.Start()
	acc := run.NewAccumulator(rr)
	stdin := bufio.NewReader(os.Stdin)
	counts := make(map[string]int)
	for {
		c := make(chan struct{})
		go func() {
			rr.Reload()
			close(c)
		}()
		<-c

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
			fmt.Printf("%v <%v>: n=%v, meta=%v\n", tag, mds[tag].GetDataClass(), counts[tag], s)
			sample := acc.Sample(tag)
			if len(sample) > 0 {
				firstStep := sample[0].EventStep
				lastStep := sample[len(sample)-1].EventStep
				fmt.Printf("\tn=%d, firstStep=%d, lastStep=%d\n", len(sample), firstStep, lastStep)
			}
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
	if err := rr.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "closing run reader: %v\n", err)
		os.Exit(1)
	}
}

func ProcessOneRecord(r io.Reader) {
	var state *tbio.TFRecordState
	fileDone := false
	r = io.MultiReader(r, alertingReader{&fileDone})
	for {
		rec, err := tbio.ReadRecord(&state, &io.LimitedReader{R: r, N: 8192})
		if err == io.EOF {
			if fileDone {
				fmt.Println("all done")
				os.Exit(0)
			}
			fmt.Println("partial chunk...")
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "reading record: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("got record of %v bytes; cksumError: %v\n", len(rec.Data), rec.Checksum())
		return
	}
}

// alertingReader is an io.Reader that always returns EOF, but flips a boolean
// flag to true when it's read from.
type alertingReader struct {
	flag *bool
}

func (r alertingReader) Read([]byte) (int, error) {
	*r.flag = true
	return 0, io.EOF
}
