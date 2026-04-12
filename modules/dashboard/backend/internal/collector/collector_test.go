package collector

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// --- fake collector ---

type fakeCollector struct {
	name  string
	tier  Tier
	err   error
	onRun func()
	runs  atomic.Int64
}

func (f *fakeCollector) Name() string { return f.name }
func (f *fakeCollector) Tier() Tier   { return f.tier }

func (f *fakeCollector) Run(_ context.Context) error {
	f.runs.Add(1)
	if f.onRun != nil {
		f.onRun()
	}
	return f.err
}

// --- tests ---

func TestTierInterval(t *testing.T) {
	tests := []struct {
		tier Tier
		want time.Duration
	}{
		{Hot, 2 * time.Second},
		{Medium, 5 * time.Second},
		{Cold, 30 * time.Second},
	}
	for _, tt := range tests {
		if got := tt.tier.Interval(); got != tt.want {
			t.Errorf("Tier(%d).Interval() = %v, want %v", tt.tier, got, tt.want)
		}
	}
}

func TestTierString(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{Hot, "hot"},
		{Medium, "medium"},
		{Cold, "cold"},
	}
	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}

func TestRunnerRunsOnceOnStart(t *testing.T) {
	ran := make(chan struct{}, 1)
	fc := &fakeCollector{
		name: "eager",
		tier: Hot,
		onRun: func() {
			select {
			case ran <- struct{}{}:
			default:
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := NewRunner(nil, []Collector{fc})
	r.Start(ctx)

	select {
	case <-ran:
		// OK -- Run was called eagerly on start.
	case <-time.After(3 * time.Second):
		t.Fatal("collector was not called within 3s of Start")
	}

	cancel()
	r.Wait()
}

func TestRunnerContinuesOnFailure(t *testing.T) {
	fc := &fakeCollector{
		name: "failing",
		tier: Hot,
		err:  errors.New("boom"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := NewRunner(nil, []Collector{fc})
	r.Start(ctx)

	// Wait long enough for at least 2 ticks (Hot=2s, so ~5s is plenty).
	time.Sleep(5 * time.Second)

	if n := fc.runs.Load(); n < 2 {
		t.Fatalf("expected at least 2 runs despite errors, got %d", n)
	}

	cancel()
	r.Wait()
}

func TestRunnerShutdownOnContextCancel(t *testing.T) {
	fc := &fakeCollector{
		name: "shutdown",
		tier: Hot,
	}

	ctx, cancel := context.WithCancel(context.Background())

	r := NewRunner(nil, []Collector{fc})
	r.Start(ctx)

	// Let it tick once.
	time.Sleep(100 * time.Millisecond)

	cancel()

	done := make(chan struct{})
	go func() {
		r.Wait()
		close(done)
	}()

	select {
	case <-done:
		// OK -- Wait returned after cancel.
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not return within 3s after context cancel")
	}
}
