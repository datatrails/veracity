package veracity

import (
	"errors"

	"github.com/gosuri/uiprogress"
)

var (
	ErrProgressStepInvalid = errors.New("the reported progress step is invalid")
)

type noopStagedProgressor struct{}

func (p *noopStagedProgressor) Completed(string, bool) error {
	return nil
}

func (p *noopStagedProgressor) Current() string { return "" }
func (p *noopStagedProgressor) ICurrent() int   { return 0 }
func (p *noopStagedProgressor) Steps() []string { return nil }

type Progresser interface {
	// Completed advances the progress bar up to the next named step.
	// If forceNext is true, all remaining steps are completed regardless of the step name
	// If step is *before* the current or is not found, an  error is returned.
	Completed(string, bool) error
	Current() string
	ICurrent() int
	Steps() []string
}

func NewNoopProgress() Progresser {
	return &noopStagedProgressor{}
}

type progress struct {
	stages []string
	// bar represents progress towards completion of massifs for a single tenant
	bar *uiprogress.Bar
}

func NewStagedProgress(prefix string, count int, steps []string) *progress {
	increments := count
	if len(steps) > 0 {
		increments = count * len(steps)
	}
	return &progress{
		stages: steps,
		bar: uiprogress.AddBar(increments).PrependElapsed().PrependFunc(func(b *uiprogress.Bar) string {
			if len(steps) > 0 {
				// return tenant + ": " + steps[(b.Current()%count)-1]
				cur := b.Current()
				i := ((cur + 1) % increments) - 1
				return prefix + ": " + steps[i]
			}
			return prefix
		}),
	}
}

// Current returns the name of the current step.
func (p *progress) Current() string { return p.stages[p.ICurrent()] }

// ICurrent returns the index of the current step.
func (p *progress) ICurrent() int { return (p.bar.Current() % (p.bar.Total / len(p.stages))) - 1 }

// Steps returns the names of the steps.
func (p *progress) Steps() []string { return p.stages[:] }

// Completed advances the progress bar up to the next named step.
// If forceNext is true, all remaining steps are completed regardless of the step name
// If step is *before* the current or is not found, an  error is returned.
func (p *progress) Completed(step string, forceNext bool) error {
	if len(p.stages) == 0 {
		p.bar.Incr()
		return nil
	}
	cur := ((p.bar.Current()+1) % (p.bar.Total / len(p.stages)))

	var next int
	for next = 0; next < len(p.stages); next++ {
		if step == p.stages[next] {
			break
		}
	}
	if forceNext {
		next = len(p.stages) - 1
	}
	if next < cur || next == len(p.stages) {
		return ErrProgressStepInvalid
	}
	if next == cur {
		return nil
	}
	for ; cur <= next; cur++ {
		p.bar.Incr()
	}
	return nil
}
