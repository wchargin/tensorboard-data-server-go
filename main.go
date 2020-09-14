package main

import (
	"fmt"

	"github.com/tensorflow/tensorflow/tensorflow/go/core/util/event_go_proto"
	"github.com/wchargin/tensorboard-data-server/mem"
)

func main() {
	rsv := mem.NewEagerReservoir(10)
	for i := int64(0); i < 50; i++ {
		step := i
		if step > 30 {
			step -= 10
		}
		rsv.Offer(&Event{event_go_proto.Event{Step: step, WallTime: 0.0}})
		for _, v := range rsv.Sample() {
			fmt.Printf("{%v} ", v)
		}
		fmt.Println()
	}
}

type Event struct {
	event_go_proto.Event
}

func (e *Event) Step() mem.Step {
	return mem.Step(e.Event.Step)
}
