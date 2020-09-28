package eventfile

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/runtime/protoiface"

	spb "github.com/tensorflow/tensorflow/tensorflow/go/core/framework/summary_go_proto"
	epb "github.com/tensorflow/tensorflow/tensorflow/go/core/util/event_go_proto"
	tbio "github.com/wchargin/tensorboard-data-server/io"
)

func TestGrowingEventFile(t *testing.T) {
	var buf bytes.Buffer

	input1 := &epb.Event{What: &epb.Event_FileVersion{FileVersion: "brain.Event:2"}}
	input2 := &epb.Event{
		What: &epb.Event_Summary{
			Summary: &spb.Summary{
				Value: []*spb.Summary_Value{
					{
						Tag: "loss",
						Value: &spb.Summary_Value_SimpleValue{
							SimpleValue: 0.1,
						},
					},
				},
			},
		},
	}

	rec1 := tbio.NewTFRecord(marshalHard(t, input1))
	rec1.Write(&buf)
	rec2 := tbio.NewTFRecord(marshalHard(t, input2))
	rec2.Write(&buf)
	rec2.Write(&buf) // again!

	// Start with a buffer that has the entire first record and a truncated
	// prefix of the second record. After this goes to sleep, fill up the
	// rest of the buffer.
	truncateAfter := 7
	split := rec1.ByteSize() + truncateAfter
	buf1 := bytes.NewBuffer(append([]byte{}, buf.Bytes()[:split]...))
	buf2 := bytes.NewBuffer(append([]byte{}, buf.Bytes()[split:]...))
	buf.Reset()
	buf.ReadFrom(buf1)

	efr := ReaderBuilder{File: &buf}.Start()
	efr.Wake <- Resume

	// First read should read a full record.
	select {
	case got := <-efr.Results:
		want := EventResult{Event: input1}
		if !proto.Equal(got.Event, want.Event) || got.Err != nil || got.Fatal {
			t.Errorf("first read: got %+v, want %+v", got, want)
		}
	case <-efr.Asleep:
		t.Fatalf("got Asleep, want first result")
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want first result")
	}

	// Second read should progress partially, then sleep due to truncation.
	select {
	case got := <-efr.Results:
		t.Fatalf("unexpected result: got %+v, want first sleep", got)
	case <-efr.Asleep:
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want first sleep")
	}

	// Wake, even though no new data has been written yet. Should go right
	// back to sleep.
	efr.Wake <- Resume
	select {
	case got := <-efr.Results:
		t.Fatalf("unexpected result: got %+v, want second sleep", got)
	case <-efr.Asleep:
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want second sleep")
	}

	// Wake with new data. Should resume from previous truncation.
	buf.ReadFrom(buf2)
	efr.Wake <- Resume
	select {
	case got := <-efr.Results:
		want := EventResult{Event: input2}
		if !proto.Equal(got.Event, want.Event) || got.Err != nil || got.Fatal {
			t.Errorf("second read: got %+v, want %+v", got, want)
		}
	case <-efr.Asleep:
		t.Fatalf("got Asleep, want second result")
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want second result")
	}

	// Third read is another full event (identical to the second one).
	select {
	case got := <-efr.Results:
		want := EventResult{Event: input2}
		if !proto.Equal(got.Event, want.Event) || got.Err != nil || got.Fatal {
			t.Errorf("third read: got %+v, want %+v", got, want)
		}
	case <-efr.Asleep:
		t.Fatalf("got Asleep, want third result")
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want third result")
	}

	select {
	case got := <-efr.Results:
		t.Fatalf("unexpected result: got %+v, want third sleep", got)
	case <-efr.Asleep:
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want third sleep")
	}
}

func TestEventFileWithBadRecordLength(t *testing.T) {
	var buf bytes.Buffer

	inputEvent := &epb.Event{What: &epb.Event_FileVersion{FileVersion: "brain.Event:2"}}
	okRecord := tbio.NewTFRecord(marshalHard(t, inputEvent))
	okRecord.Write(&buf)

	// Write an all-zeros record, which is corrupt: length checksum wrong.
	emptyRecord := tbio.NewTFRecord([]byte{})
	buf.Write(make([]byte, emptyRecord.ByteSize()))

	okRecord.Write(&buf)

	efr := ReaderBuilder{File: &buf}.Start()
	efr.Wake <- Resume

	// First read should succeed.
	select {
	case got := <-efr.Results:
		want := EventResult{Event: inputEvent}
		if !proto.Equal(got.Event, want.Event) || got.Err != nil || got.Fatal {
			t.Errorf("first read: got %+v, want %+v", got, want)
		}
	case <-efr.Asleep:
		t.Fatalf("got Asleep, want first result")
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want first result")
	}

	// Second read should fail fatally.
	select {
	case got := <-efr.Results:
		wantMsgSubstr := "length CRC mismatch"
		if got.Event != nil || got.Err == nil || !strings.Contains(got.Err.Error(), wantMsgSubstr) || !got.Fatal {
			t.Errorf("first read: got %+v, want fatal failure with %q", got, wantMsgSubstr)
		}
	case <-efr.Asleep:
		t.Fatalf("got Asleep, want second result")
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want second result")
	}

	// Reader should now be dead.
	select {
	case got := <-efr.Results:
		t.Errorf("got result %+v, want no interaction", got)
	case <-efr.Asleep:
		t.Errorf("got <-Asleep, want no interaction")
	case efr.Wake <- Resume:
		t.Errorf("got Wake<-, want no interaction")
	default:
	}
}

func TestEventFileWithBadRecordData(t *testing.T) {
	var buf bytes.Buffer

	inputEvent := &epb.Event{What: &epb.Event_FileVersion{FileVersion: "brain.Event:2"}}
	okRecord := tbio.NewTFRecord(marshalHard(t, inputEvent))
	okRecord.Write(&buf)
	buf.Bytes()[okRecord.ByteSize()-1] ^= 0x55
	okRecord.Write(&buf)

	efr := ReaderBuilder{File: &buf}.Start()
	efr.Wake <- Resume

	// First read should fail non-fatally.
	select {
	case got := <-efr.Results:
		wantMsgSubstr := "data CRC mismatch"
		if got.Event != nil || got.Err == nil || !strings.Contains(got.Err.Error(), wantMsgSubstr) || got.Fatal {
			t.Errorf("first read: got %+v, want non-fatal failure with %q", got, wantMsgSubstr)
		}
	case <-efr.Asleep:
		t.Fatalf("got Asleep, want first result")
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want first result")
	}

	// Second read should succeed.
	select {
	case got := <-efr.Results:
		want := EventResult{Event: inputEvent}
		if !proto.Equal(got.Event, want.Event) || got.Err != nil || got.Fatal {
			t.Errorf("second read: got %+v, want %+v", got, want)
		}
	case <-efr.Asleep:
		t.Fatalf("got Asleep, want second result")
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want second result")
	}

	select {
	case got := <-efr.Results:
		t.Fatalf("unexpected result: got %+v, want sleep", got)
	case <-efr.Asleep:
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want sleep")
	}
}

func TestEventFileWithBadProto(t *testing.T) {
	var buf bytes.Buffer

	badRecord := tbio.NewTFRecord([]byte("not likely a proto"))
	badRecord.Write(&buf)
	inputEvent := &epb.Event{What: &epb.Event_FileVersion{FileVersion: "brain.Event:2"}}
	okRecord := tbio.NewTFRecord(marshalHard(t, inputEvent))
	okRecord.Write(&buf)

	efr := ReaderBuilder{File: &buf}.Start()
	efr.Wake <- Resume

	// First read should fail non-fatally.
	select {
	case got := <-efr.Results:
		wantMsgSubstr := "reserved wire type"
		if got.Event != nil || got.Err == nil || !strings.Contains(got.Err.Error(), wantMsgSubstr) || got.Fatal {
			t.Errorf("first read: got %+v, want non-fatal failure with %q", got, wantMsgSubstr)
		}
	case <-efr.Asleep:
		t.Fatalf("got Asleep, want first result")
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want first result")
	}

	// Second read should succeed.
	select {
	case got := <-efr.Results:
		want := EventResult{Event: inputEvent}
		if !proto.Equal(got.Event, want.Event) || got.Err != nil || got.Fatal {
			t.Errorf("second read: got %+v, want %+v", got, want)
		}
	case <-efr.Asleep:
		t.Fatalf("got Asleep, want second result")
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want second result")
	}

	select {
	case got := <-efr.Results:
		t.Fatalf("unexpected result: got %+v, want sleep", got)
	case <-efr.Asleep:
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want sleep")
	}
}

func TestWakeAbort(t *testing.T) {
	var buf bytes.Buffer

	efr := ReaderBuilder{File: &buf}.Start()
	efr.Wake <- Resume

	select {
	case got := <-efr.Results:
		t.Fatalf("unexpected result: got %+v, want first sleep", got)
	case <-efr.Asleep:
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want first sleep")
	}

	efr.Wake <- Resume

	select {
	case got := <-efr.Results:
		t.Fatalf("unexpected result: got %+v, want second sleep", got)
	case <-efr.Asleep:
	case <-time.After(time.Second):
		t.Fatalf("no interaction after 1s; want second sleep")
	}

	efr.Wake <- Abort

	select {
	case got := <-efr.Results:
		t.Fatalf("unexpected result: got %+v, want dead", got)
	case <-efr.Asleep:
		t.Fatalf("unexpected result: got sleep, want dead")
	default:
	}
}

func TestImmediateAbort(t *testing.T) {
	bufContents := "do not read me"
	buf := bytes.NewBufferString(bufContents)

	efr := ReaderBuilder{File: buf}.Start()
	efr.Wake <- Abort

	if buf.String() != bufContents {
		t.Errorf("buf.String(): got %v, want %v", buf.String(), bufContents)
	}
}

// marshalHard calls proto.Marshal and fails the test in case of error.
func marshalHard(t *testing.T, m protoiface.MessageV1) []byte {
	result, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("proto.Marshal(%v): %v", m, err)
	}
	return result
}
