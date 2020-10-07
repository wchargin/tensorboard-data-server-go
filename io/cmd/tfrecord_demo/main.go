package main

import (
	"fmt"
	"io"
	"os"

	tbio "github.com/wchargin/tensorboard-data-server/io"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "must specify event file path as first argument\n")
		os.Exit(1)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	for {
		processOneRecord(f)
	}
}

func processOneRecord(r io.Reader) {
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
