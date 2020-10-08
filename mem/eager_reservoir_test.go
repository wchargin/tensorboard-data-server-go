package mem

import (
	"testing"
)

// JustStep is a simple StepIndexed value with no extra information.
type JustStep struct {
	step Step
}

func (x JustStep) Step() Step {
	return x.step
}

func extractSteps(rsv EagerReservoir) []Step {
	sample := rsv.Sample()
	result := make([]Step, len(sample))
	for i, v := range sample {
		result[i] = v.Step()
	}
	return result
}

func stepsEqual(s1, s2 []Step) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true

}

func TestSimple(t *testing.T) {
	rsv := NewEagerReservoir(10)

	if sample := rsv.Sample(); len(sample) != 0 {
		t.Errorf("empty reservoir: Sample(): got %v, want empty array", sample)
	}
	if last := rsv.Last(); last != nil {
		t.Errorf("empty reservoir: Last(): got %v, want nil", last)
	}

	// Fill with `[i * i for i in range(10)]`, exactly filling the reservoir.
	for i := 1; i <= 10; i++ {
		rsv.Offer(JustStep{step: Step(i * i)})
		expectedSteps := make([]Step, i)
		for j := 1; j <= i; j++ {
			expectedSteps[j-1] = Step(j * j)
		}
		if s := extractSteps(rsv); !stepsEqual(s, expectedSteps) {
			t.Errorf("i=%v: got %v, want %v", i, s, expectedSteps)
		}
		if got, want := rsv.Last(), Step(i*i); got == nil || got.Step() != want {
			t.Errorf("i=%v: got last=%v, want step %v", i, got, want)
		}
	}

	// Fill with more square numbers, keeping last but not overflowing.
	for i := 11; i <= 16; i++ {
		rsv.Offer(JustStep{step: Step(i * i)})
		steps := extractSteps(rsv)
		if len(steps) != 10 {
			t.Errorf("i=%v: got %v, wanted length 10", i, steps)
		} else if steps[len(steps)-1] != Step(i*i) {
			t.Errorf("i=%v: got %v, wanted [..., %v]", i, steps, Step(i*i))
		}
		if got, want := rsv.Last(), Step(i*i); got == nil || got.Step() != want {
			t.Errorf("i=%v: got last=%v, want step %v", i, got, want)
		}
	}

	// Seen 16 records, keeping 10. Preempt to invalidate records 9..=16,
	// that the reservoir must have between 2 and 8 old records before the
	// new one is added.
	rsv.Offer(JustStep{step: 70}) // 8*8 < 70 < 9*9
	{
		steps := extractSteps(rsv)
		if len(steps) < 2 || len(steps) > 8+1 {
			t.Errorf("after preemption: got %v, wanted 2<=len<=9", steps)
		}
		{
			sample := rsv.Sample()
			if got, want := rsv.Last(), sample[len(sample)-1]; got != want {
				t.Errorf("after preemption: got %v, want match %v", got, want)
			}
		}

		increasing := true
		for i := 0; i < len(steps)-1; i++ {
			if steps[i+1] <= steps[i] {
				increasing = false
			}
		}
		if !increasing {
			t.Errorf("after preemption: got %v, wanted increasing", steps)
		}

		inRange := true
		for i := 0; i < len(steps)-1; i++ {
			if steps[i] > 64 {
				inRange = false
			}
		}
		if !inRange {
			t.Errorf("after preemption: got %v, wanted all-but-last < 70", steps)
		}

		if steps[len(steps)-1] != 70 {
			t.Errorf("after preemption: got %v, wanted last = 70", steps)
		}
	}

	// One more sanity check: add another record. The "70" preemption may
	// or may not be evicted, but this new record should be the last.
	rsv.Offer(JustStep{step: 71})
	if steps := extractSteps(rsv); steps[len(steps)-1] != 71 {
		t.Errorf("after final record: got Sample() %v, wanted last = 71", steps)
	}
	if last := rsv.Last(); last.Step() != 71 {
		t.Errorf("after final record: got Last() %v, wanted last = 71", last)
	}
}

func TestDeterministic(t *testing.T) {
	r1 := NewEagerReservoir(10)
	r2 := NewEagerReservoir(10)

	for i := 0; i < 100; i++ {
		// Make sure that the samples so far agree.
		if s1, s2 := extractSteps(r1), extractSteps(r2); !stepsEqual(s1, s2) {
			t.Errorf("i=%v: s1 != s2: %v != %v", i, s1, s2)
		}

		// Add one more point, with a preemption after 60 steps.
		step := i
		if step >= 60 {
			step -= 20
		}
		r1.Offer(JustStep{step: Step(step)})
		r2.Offer(JustStep{step: Step(step)})
	}
}
