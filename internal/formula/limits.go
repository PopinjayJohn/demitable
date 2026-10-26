package formula

import "time"

// Limits bounds evaluation.
type Limits struct {
	MaxDepth   int           // max recursion depth
	MaxSteps   int           // max evaluation steps
	Timeout    time.Duration // max wall time; 0 means default
	DepthLimit int
	StepLimit  int
	TimeLimit  time.Duration
}

// DefaultLimits returns sensible defaults.
func DefaultLimits() Limits {
	return Limits{MaxDepth: 256, MaxSteps: 100000, Timeout: 2 * time.Second}
}

func (l Limits) normalize() Limits {
	d := DefaultLimits()
	if l.MaxDepth <= 0 {
		l.MaxDepth = l.DepthLimit
	}
	if l.MaxDepth <= 0 {
		l.MaxDepth = d.MaxDepth
	}
	if l.MaxSteps <= 0 {
		l.MaxSteps = l.StepLimit
	}
	if l.MaxSteps <= 0 {
		l.MaxSteps = d.MaxSteps
	}
	if l.Timeout <= 0 {
		l.Timeout = l.TimeLimit
	}
	if l.Timeout <= 0 {
		l.Timeout = d.Timeout
	}
	return l
}
