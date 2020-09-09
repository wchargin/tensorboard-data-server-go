package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"

	tbio "github.com/wchargin/tensorboard-data-server/io"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

var bufsize = flag.Int("bufsize", 8192, "bufio.NewReaderSize capacity")

var checksum = flag.Bool("checksum", false, "validate TFRecord payloads against CRC-32")

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "must specify event file path as only argument\n")
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

	ReadAllRecords(args[0])

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

func ReadAllRecords(fileName string) {
	rawFile, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	f := bufio.NewReaderSize(rawFile, *bufsize)
	recordsRead := 0
	totalPayloadSize := 0
	for {
		payloadSize, more := ProcessOneRecord(f)
		recordsRead++
		totalPayloadSize += payloadSize
		if !more {
			break
		}
	}
	fmt.Printf("all done; read %v records (%v bytes payload)\n", recordsRead, totalPayloadSize)
}

func ProcessOneRecord(r io.Reader) (payloadSize int, more bool) {
	var state *tbio.TFRecordState
	rec, err := tbio.ReadRecord(&state, r)
	if err == io.EOF {
		return 0, false
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading record: %v\n", err)
		os.Exit(1)
	}
	if *checksum {
		if err := rec.Checksum(); err != nil {
			fmt.Fprintf(os.Stderr, "checksum failure: %v\n", err)
		}
	}
	return len(rec.Data), true
}
