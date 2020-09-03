package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	logdirFlag := flag.String("logdir", "", "Top-level directory containing event files")
	flag.Parse()

	logdir := *logdirFlag
	if len(logdir) == 0 {
		fmt.Fprintln(os.Stderr, "fatal: no --logdir given")
		os.Exit(1)
	}
	fmt.Printf("Logdir: %q\n", logdir)
}
