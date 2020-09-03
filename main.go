package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wchargin/tensorboard-data-server/mem"
)

func main() {
	logdirFlag := flag.String("logdir", "", "Top-level directory containing event files")
	flag.Parse()

	rsv := mem.NewEagerReservoir(10)
	for i := int64(0); i < 50; i++ {
		step := i
		if step > 30 {
			step -= 10
		}
		rsv.Offer(Scalar{step: mem.Step(step), wallTime: 0.0, value: float64(step)})
		for _, v := range rsv.Sample() {
			fmt.Printf("%v ", v.(Scalar).value)
		}
		fmt.Println()
	}

	logdir := *logdirFlag
	if len(logdir) == 0 {
		fmt.Fprintln(os.Stderr, "fatal: no --logdir given")
		os.Exit(1)
	}
	fmt.Printf("Logdir: %q\n", logdir)
}

type Scalar struct {
	step     mem.Step
	wallTime float64
	value    float64
}

func (d Scalar) Step() mem.Step {
	return d.step
}
