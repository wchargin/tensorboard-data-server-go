package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"

	"github.com/wchargin/tensorboard-data-server/mem"
)

type blobKey struct {
	eid   string
	run   string
	tag   string
	step  mem.Step
	index int64
}

var b64Enc = base64.RawURLEncoding

// An encodedBlobKey is a UTF-8, URL-safe string representing a blobKey.
//
// Implementation: padding-stripped base64 over JSON of wireBlobKey.
type encodedBlobKey string

// A wireBlobKey holds the fields of blobKey in declaration order, but strings
// are base64-encoded for losslessness since Go's json.Marshal of strings is
// lossy. Can't just cast to []byte because then Unmarshal will deserialize
// them as strings since wireBlobKey only has interface{} type.
type wireBlobKey [5]interface{}

func (bk *blobKey) encode() encodedBlobKey {
	wire := wireBlobKey{
		b64Enc.EncodeToString([]byte(bk.eid)),
		b64Enc.EncodeToString([]byte(bk.run)),
		b64Enc.EncodeToString([]byte(bk.tag)),
		bk.step,
		bk.index,
	}
	jsonBuf, err := json.Marshal(wire)
	if err != nil {
		log.Fatalf("(*blobKey).encode: json.Marshal(%+v) = %q, %#v", bk, jsonBuf, err)
	}
	return encodedBlobKey(b64Enc.EncodeToString(jsonBuf))
}

func decodeBlobKey(k string) (*blobKey, error) {
	b64Buf, err := b64Enc.DecodeString(k)
	if err != nil {
		return nil, err
	}
	var wire wireBlobKey
	if err := json.Unmarshal(b64Buf, &wire); err != nil {
		return nil, err
	}

	bk := new(blobKey)
	if eid, err := decodeWireString("eid", wire[0]); err != nil {
		return nil, err
	} else {
		bk.eid = eid
	}
	if run, err := decodeWireString("run", wire[1]); err != nil {
		return nil, err
	} else {
		bk.run = run
	}
	if tag, err := decodeWireString("tag", wire[2]); err != nil {
		return nil, err
	} else {
		bk.tag = tag
	}
	if step, err := decodeWireInt64("step", wire[3]); err != nil {
		return nil, err
	} else {
		bk.step = mem.Step(step)
	}
	if index, err := decodeWireInt64("index", wire[4]); err != nil {
		return nil, err
	} else {
		bk.index = index
	}
	return bk, nil
}

func decodeWireString(what string, x interface{}) (string, error) {
	s, ok := x.(string)
	if !ok {
		return "", fmt.Errorf("%v: got %T, want string", what, x)
	}
	b64Buf, err := b64Enc.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b64Buf), nil
}

func decodeWireInt64(what string, x interface{}) (int64, error) {
	f, ok := x.(float64)
	if !ok {
		return 0, fmt.Errorf("%v: got %T, want integral float64", what, x)
	}
	intPart, fracPart := math.Modf(f)
	if fracPart != 0.0 {
		return 0, fmt.Errorf("%v: got %v, want integer", what, f)
	}
	i64 := int64(intPart)
	if float64(i64) != intPart {
		return 0, fmt.Errorf("%v: got %v, want int64", what, f)
	}
	return i64, nil
}
