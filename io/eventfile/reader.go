package eventfile

import (
	"io"

	"github.com/golang/protobuf/proto"

	"github.com/tensorflow/tensorflow/tensorflow/go/core/util/event_go_proto"
	tbio "github.com/wchargin/tensorboard-data-server/io"
)

// EventResult describes the result of attempting to read an event, which may
// have failed. Event != nil if and only if Err == nil. If Err == nil, then
// Fatal == false. If Fatal is true, then the event file reader is dead.
type EventResult struct {
	Event *event_go_proto.Event
	Err   error
	Fatal bool
}

// A WakeAction tells a Reader what to do after waking up.
type WakeAction int

const (
	// Resume indicates that the reader should keep reading records.
	Resume WakeAction = iota
	// Abort indicates that the reader should discard its state and exit
	// immediately.
	Abort
)

// ReaderBuilder specifies options for a Reader.
type ReaderBuilder struct {
	// File is the input stream for the event file.
	File io.Reader
}

type readerState struct {
	// Results is the input end of Reader.Results.
	Results chan<- EventResult
	// Results is the input end of Reader.Asleep.
	Asleep chan<- bool
	// Wake is the output end of Reader.Wake.
	Wake <-chan WakeAction
}

// Reader reads TFRecords from an event file and parses them as Event protos.
// It expects that the file that it's reading is being actively written, and as
// such merely dozes off at EOF rather than exiting entirely. It can be awoken
// by its owner later: e.g., after some amount of time has passed.
type Reader struct {
	// Results is an output channel for each event read from the file. If
	// this ever emits a fatal error, the owner should expect all future
	// interactions with these channels to block forever.
	Results <-chan EventResult
	// Asleep is an output channel that sees "true" when the file has been
	// read to its end, for now. It never sees "false".
	Asleep <-chan bool
	// Wake is an input channel that sees a wake action when it should stop
	// being asleep.
	Wake chan<- WakeAction
}

// Start starts a reader in a new goroutine. It reads the full contents of the
// event file, then goes to sleep until woken, then starts again in a loop.
func (b ReaderBuilder) Start() *Reader {
	results := make(chan EventResult)
	asleep := make(chan bool)
	wake := make(chan WakeAction)
	rs := &readerState{Results: results, Asleep: asleep, Wake: wake}
	go rs.start(b.File)
	return &Reader{Results: results, Asleep: asleep, Wake: wake}
}

func (efr *readerState) start(file io.Reader) {
	var recordState *tbio.TFRecordState
	for {
		record, err := tbio.ReadRecord(&recordState, file)
		if err == io.EOF {
			efr.Asleep <- true
			switch <-efr.Wake {
			case Resume:
				continue
			case Abort:
				return
			}
		}
		if err != nil {
			efr.Results <- EventResult{Err: err, Fatal: true}
			return
		}
		recordState = nil
		event, err := efr.readEvent(record)
		if err != nil {
			efr.Results <- EventResult{Err: err, Fatal: false}
			continue
		}
		efr.Results <- EventResult{Event: event}
	}
}

func (efr *readerState) readEvent(record *tbio.TFRecord) (*event_go_proto.Event, error) {
	if err := record.Checksum(); err != nil {
		return nil, err
	}
	var event event_go_proto.Event
	if err := proto.Unmarshal(record.Data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
