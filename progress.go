package veracity

import (
	"github.com/gosuri/uiprogress"
)

type noopStagedProgressor struct{}

func (p *noopStagedProgressor) Completed() {}

func (p *noopStagedProgressor) Current() int { return 0 }

type Progresser interface {
	Completed()
}

func NewNoopProgress() Progresser {
	return &noopStagedProgressor{}
}

type progress struct {
	// bar represents progress towards completion of massifs for a single tenant
	bar *uiprogress.Bar
}

func NewStagedProgress(prefix string, count int) Progresser {
	if count == 0 {
		return NewNoopProgress()
	}
	return &progress{
		bar: uiprogress.AddBar(count).PrependElapsed().PrependFunc(func(b *uiprogress.Bar) string {
			return prefix + ":"
		}),
	}
}

// Current returns the index of the current step.
func (p *progress) Current() int { return p.bar.Current() }

// Completed advances the progress bar one increment
func (p *progress) Completed() {
	p.bar.Incr()
}
