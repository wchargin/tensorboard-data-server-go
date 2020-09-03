package mem

// Type Step represents a strictly monotonically increasing step value. It is
// stored as an int64, but must be non-negative.
type Step int64

// Interface StepIndexed describes values that are uniquely indexed by "step",
// a non-negative integer that increases with time (potentially
// non-consecutively).
type StepIndexed interface {
	// Step returns the step for this value, which must be non-negative.
	// See docs for StepIndexed for a description of steps.
	Step() Step
}
