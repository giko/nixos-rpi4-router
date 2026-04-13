package collector

import (
	"context"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/proc"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/systemd"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/vcgencmd"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

// SystemOpts configures the System collector.
type SystemOpts struct {
	StatPath    string
	MeminfoPath string
	ThermalPath string
	UptimePath  string
	State       *state.State
}

// System reads /proc/stat, /proc/meminfo, thermal zone, and /proc/uptime,
// computes CPU percentages from deltas, and publishes via
// State.SetSystemHot so it never touches the medium-tier fields.
type System struct {
	opts    SystemOpts
	lastCPU *proc.CPUTimes
}

// NewSystem creates a System collector.
func NewSystem(opts SystemOpts) *System {
	return &System{opts: opts}
}

func (*System) Name() string { return "system" }
func (*System) Tier() Tier   { return Hot }

// Run performs a single collection pass.
func (s *System) Run(_ context.Context) error {
	stat, err := proc.ReadStat(s.opts.StatPath)
	if err != nil {
		return err
	}

	mem, err := proc.ReadMeminfo(s.opts.MeminfoPath)
	if err != nil {
		return err
	}

	temp, err := proc.ReadThermal(s.opts.ThermalPath)
	if err != nil {
		return err
	}

	uptime, err := proc.ReadUptime(s.opts.UptimePath)
	if err != nil {
		return err
	}

	// Compute CPU percentages from delta (zero on first run).
	var cpu model.CPUStats
	if s.lastCPU != nil {
		d := stat.CPU.Delta(*s.lastCPU)
		total := d.Total()
		if total > 0 {
			ft := float64(total)
			cpu = model.CPUStats{
				PercentUser:   float64(d.User+d.Nice) / ft * 100,
				PercentSystem: float64(d.System) / ft * 100,
				PercentIdle:   float64(d.Idle) / ft * 100,
				PercentIOWait: float64(d.IOWait) / ft * 100,
			}
		}
	}
	s.lastCPU = &stat.CPU

	usedBytes := mem.TotalBytes - mem.AvailableBytes
	var pctUsed float64
	if mem.TotalBytes > 0 {
		pctUsed = float64(usedBytes) / float64(mem.TotalBytes) * 100
	}

	// Write only hot-tier fields under the state lock so a concurrent
	// medium-tier update can't be clobbered by a stale snapshot.
	s.opts.State.SetSystemHot(
		cpu,
		model.MemoryStats{
			TotalBytes:     mem.TotalBytes,
			AvailableBytes: mem.AvailableBytes,
			UsedBytes:      usedBytes,
			PercentUsed:    pctUsed,
		},
		temp,
		uptime,
		time.Unix(int64(stat.BootTimeUnix), 0).UTC(),
	)
	return nil
}

// --------------- SystemMedium collector ---------------

// SystemMediumOpts configures the SystemMedium collector.
type SystemMediumOpts struct {
	Units []string
	State *state.State
}

// SystemMedium collects service states (via systemctl) and throttle flags
// (via vcgencmd) on a medium-tier (5 s) interval.
type SystemMedium struct {
	opts SystemMediumOpts
}

// NewSystemMedium creates a SystemMedium collector.
func NewSystemMedium(opts SystemMediumOpts) *SystemMedium {
	return &SystemMedium{opts: opts}
}

func (*SystemMedium) Name() string { return "system-medium" }
func (*SystemMedium) Tier() Tier   { return Medium }

// Run performs a single collection pass. It collects medium-tier fields
// (services, throttle) and writes them via SetSystemMedium, which updates
// only those fields under the state mutex so hot-tier fields stay intact.
func (m *SystemMedium) Run(ctx context.Context) error {
	services, svcErr := systemd.Collect(ctx, m.opts.Units)
	throttle, tFlag, tErr := vcgencmd.Collect(ctx)

	// If both sources failed, return first error. Nothing is written to
	// state so the existing medium values (and their updated_at) keep
	// surfacing staleness to the handler.
	if svcErr != nil && tErr != nil {
		return svcErr
	}

	// Pull the previous medium-tier fields so partial failures preserve
	// whichever half we couldn't refresh this tick. We read under the
	// state mutex via SnapshotSystem; only the medium fields are used.
	prev, _ := m.opts.State.SnapshotSystem()
	if svcErr != nil {
		services = prev.Services
	}
	if tErr != nil {
		throttle = prev.Throttled
		tFlag = prev.ThrottledFlag
	}

	m.opts.State.SetSystemMedium(services, throttle, tFlag)
	return nil
}
