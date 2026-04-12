package collector

import (
	"context"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/proc"
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
// computes CPU percentages from deltas, and publishes via State.SetSystem.
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

	// Preserve medium-tier fields (Services, Throttled) from existing state.
	existing, _ := s.opts.State.SnapshotSystem()

	sys := model.SystemStats{
		CPU: cpu,
		Memory: model.MemoryStats{
			TotalBytes:     mem.TotalBytes,
			AvailableBytes: mem.AvailableBytes,
			UsedBytes:      usedBytes,
			PercentUsed:    pctUsed,
		},
		TemperatureC:  temp,
		UptimeSeconds: uptime,
		BootTime:      time.Unix(int64(stat.BootTimeUnix), 0).UTC(),
		// Preserve medium-tier fields.
		Throttled:     existing.Throttled,
		ThrottledFlag: existing.ThrottledFlag,
		Services:      existing.Services,
	}

	s.opts.State.SetSystem(sys)
	return nil
}
