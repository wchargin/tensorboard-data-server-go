package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/wchargin/tensorboard-data-server/fs"
	"github.com/wchargin/tensorboard-data-server/io/logdir"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "must specify run directory as first argument\n")
		os.Exit(1)
	}
	ll := logdir.LoaderBuilder{FS: fs.OS{}, Logdir: os.Args[1]}.Start()
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
