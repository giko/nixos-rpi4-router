// Package collector defines the Collector interface and a Runner that
// executes collectors on independent tickers according to their Tier.
package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Tier controls the polling interval for a collector.
type Tier int

const (
	Hot    Tier = iota // 2 s
	Medium             // 5 s
	Cold               // 30 s
)

// Interval returns the polling duration for the tier.
func (t Tier) Interval() time.Duration {
	switch t {
	case Hot:
		return 2 * time.Second
	case Medium:
		return 5 * time.Second
	case Cold:
		return 30 * time.Second
	default:
		return 5 * time.Second
	}
}

// String returns a human-readable label for the tier.
func (t Tier) String() string {
	switch t {
	case Hot:
		return "hot"
	case Medium:
		return "medium"
	case Cold:
		return "cold"
	default:
		return "unknown"
	}
}

// Collector is implemented by each data source (traffic, system, etc.).
type Collector interface {
	Name() string
	Tier() Tier
	Run(ctx context.Context) error
}

// Runner starts one goroutine per collector, each on its own ticker.
type Runner struct {
	logger     *slog.Logger
	collectors []Collector
	wg         sync.WaitGroup
}

// NewRunner creates a Runner for the given collectors. The slice is
// shallow-copied so the caller can safely modify the original afterward.
func NewRunner(logger *slog.Logger, collectors []Collector) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	cs := make([]Collector, len(collectors))
	copy(cs, collectors)
	return &Runner{
		logger:     logger,
		collectors: cs,
	}
}

// Start launches one goroutine per collector. Each goroutine runs the
// collector eagerly on start, then on a ticker matching the collector's
// tier. Start is non-blocking.
func (r *Runner) Start(ctx context.Context) {
	for _, c := range r.collectors {
		r.wg.Add(1)
		go r.loop(ctx, c)
	}
}

// Wait blocks until all collector goroutines have exited.
func (r *Runner) Wait() {
	r.wg.Wait()
}

// loop runs a single collector: eager first execution, then ticker.
func (r *Runner) loop(ctx context.Context, c Collector) {
	defer r.wg.Done()

	r.run(ctx, c)

	ticker := time.NewTicker(c.Tier().Interval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.run(ctx, c)
		}
	}
}

// run executes a single collection pass, logging errors at Warn level.
func (r *Runner) run(ctx context.Context, c Collector) {
	if err := c.Run(ctx); err != nil {
		r.logger.Warn("collector failed",
			"collector", c.Name(),
			"tier", c.Tier().String(),
			"err", err,
		)
	}
}
