package server

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wchargin/tensorboard-data-server/mem"
)

func TestBlobKeyRoundtrip(t *testing.T) {
	cases := []struct {
		name string
		bk   blobKey
	}{
		{name: "simple", bk: blobKey{eid: "123", run: "mnist", tag: "input", step: mem.Step(777), index: 23}},
		{name: "nonUnicode", bk: blobKey{eid: "123", run: "mnist", tag: "\x00\x77\x99\xcc", step: mem.Step(777), index: 23}},
	}
	for _, c := range cases {
		encoded := c.bk.encode()
		if got, err := decodeBlobKey(string(encoded)); err != nil || *got != c.bk {
			t.Errorf("case %q: got %+v, %v; want %+v, nil", c.name, got, err, &c.bk)
		}
	}
}

func TestBlobKeyDecodeInvalid(t *testing.T) {
	var s, wantErrStr string
	var bk *blobKey
	var err error

	s = "???"
	bk, err = decodeBlobKey(s)
	if _, ok := err.(base64.CorruptInputError); err == nil || !ok {
		t.Errorf("decodeBlobKey(%q): got %+v, %#v; want base64.CorruptInputError", s, bk, err)
	}

	s = b64Enc.EncodeToString([]byte("notjson"))
	bk, err = decodeBlobKey(s)
	if _, ok := err.(*json.SyntaxError); err == nil || !ok {
		t.Errorf("decodeBlobKey(%q): got %+v, %#v; want *json.SyntaxError", s, bk, err)
	}

	s = b64Enc.EncodeToString([]byte(`{"json":true,"valid":false}`))
	bk, err = decodeBlobKey(s)
	if _, ok := err.(*json.UnmarshalTypeError); err == nil || !ok {
		t.Errorf("decodeBlobKey(%q): got %+v, %#v; want *json.UnmarshalTypeError", s, bk, err)
	}

	s = b64Enc.EncodeToString([]byte(`[false,"","",0,0]`))
	bk, err = decodeBlobKey(s)
	wantErrStr = "eid: got bool, want string"
	if err == nil || !strings.Contains(err.Error(), wantErrStr) {
		t.Errorf("decodeBlobKey(%q): got %+v, %#v; want %q", s, bk, err, wantErrStr)
	}

	s = b64Enc.EncodeToString([]byte(`["???","","",0,0]`))
	bk, err = decodeBlobKey(s)
	if _, ok := err.(base64.CorruptInputError); err == nil || !ok {
		t.Errorf("decodeBlobKey(%q): got %+v, %#v; want base64.CorruptInputError", s, bk, err)
	}

	s = b64Enc.EncodeToString([]byte(`["","","",false,0]`))
	bk, err = decodeBlobKey(s)
	wantErrStr = "step: got bool, want integral float64"
	if err == nil || !strings.Contains(err.Error(), wantErrStr) {
		t.Errorf("decodeBlobKey(%q): got %+v, %#v; want %q", s, bk, err, wantErrStr)
	}

	s = b64Enc.EncodeToString([]byte(`["","","",123.45,0]`))
	bk, err = decodeBlobKey(s)
	wantErrStr = "step: got 123.45, want integer"
	if err == nil || !strings.Contains(err.Error(), wantErrStr) {
		t.Errorf("decodeBlobKey(%q): got %+v, %#v; want %q", s, bk, err, wantErrStr)
	}

	s = b64Enc.EncodeToString([]byte(`["","","",1267650600228229401496703205376,0]`))
	bk, err = decodeBlobKey(s)
	wantErrStr = "step: got 1.2676506002282294e+30, want int64"
	if err == nil || !strings.Contains(err.Error(), wantErrStr) {
		t.Errorf("decodeBlobKey(%q): got %+v, %#v; want %q", s, bk, err, wantErrStr)
	}
}
