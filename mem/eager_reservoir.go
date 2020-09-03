package mem

import (
	"math"
	"math/rand"
	"sync"
)

// Type EagerReservoir describes the contract for a naive reservoir sampling
// algorithm (data structure) over StepIndexed values. An EagerReservoir must
// inspect every record in the stream. Preemption occurs implicitly whenever
// the Step value of a record does not increase.
type EagerReservoir interface {
	// Offer accepts a record from the stream and either inserts it into
	// the reservoir or discards it (with the caveat that the most recent
	// record is always kept).
	Offer(value StepIndexed)
	// Sample takes a simple random sample from the non-preempted records
	// of the stream read so far, chosen uniformly among samples that
	// include the most recent record. The returned buffer is owned by the
	// caller.
	Sample() []StepIndexed
}

func NewEagerReservoir(capacity uint64) EagerReservoir {
	return &eagerReservoir{
		rng: rand.New(rand.NewSource(0)),
		buf: make([]StepIndexed, capacity),
	}
}

type eagerReservoir struct {
	// rng is used for determining whether and whither a given new record
	// should be added to the reservoir, once the total number of records
	// seen no longer fits in the reservoir capacity.
	rng *rand.Rand
	// seen lists the number of non-preempted records passed in the stream
	// so far, regardless of whether they were ever added to the reservoir.
	// Usually called capital "N" in the literature.
	seen int
	// stored lists the number of records currently in buf, which are
	// always contiguous starting from index 0. May decrease when
	// preemptions occur.
	stored int
	// buf stores. The slice length is always the capacity of the
	// reservoir, but it may have a bunch of nils. Representation
	// invariant: buf is stored in step-sorted order.
	buf []StepIndexed
	// mutex protects access to all fields of the reservoir other than
	// itself.
	mutex sync.Mutex
}

func (rsv *eagerReservoir) Offer(v StepIndexed) {
	rsv.mutex.Lock()
	defer rsv.mutex.Unlock()

	rsv.lockedPreempt(v.Step())

	rsv.seen++
	// Cast from int64 to int is portable because result of Int63n
	// is always smaller than its argument, and its argument value
	// was an int.
	dst := int(rsv.rng.Int63n(int64(rsv.seen)))
	if dst >= len(rsv.buf) {
		// Keep-last only.
		rsv.buf[rsv.stored-1] = v
		return
	}
	if rsv.stored < len(rsv.buf) {
		rsv.buf[rsv.stored] = v
		rsv.stored++
		return
	}
	copy(rsv.buf[dst:rsv.stored-1], rsv.buf[dst+1:rsv.stored])
	rsv.buf[rsv.stored-1] = v
}

// lockedPreempt preempts all values from the stream whose step is not smaller
// than firstBad. rsv.mutex MUST be held when calling this method.
func (rsv *eagerReservoir) lockedPreempt(firstBad Step) {
	preemptions := 0
	for i := rsv.stored; i > 0 && rsv.buf[i-1].Step() >= firstBad; i-- {
		preemptions++
	}
	if preemptions > 0 {
		facPreempted := float64(preemptions) / float64(rsv.stored)
		rsv.stored -= preemptions
		rsv.seen = int(math.Ceil(facPreempted * float64(rsv.seen)))
	}
}

func (rsv *eagerReservoir) Sample() []StepIndexed {
	rsv.mutex.Lock()
	defer rsv.mutex.Unlock()

	result := make([]StepIndexed, rsv.stored)
	copy(result, rsv.buf)
	return result
}
