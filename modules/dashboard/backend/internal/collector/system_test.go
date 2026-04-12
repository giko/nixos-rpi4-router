package collector

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

const statFixture1 = `cpu  10000 200 3000 46000 800 0 0 0 0 0
cpu0 2500 50 750 11500 200 0 0 0 0 0
btime 1680012345
`

const statFixture2 = `cpu  10500 210 3200 46800 810 0 0 0 0 0
cpu0 2600 55 800 11700 205 0 0 0 0 0
btime 1680012345
`

const meminfoFixture = `MemTotal:        3884292 kB
MemFree:          152340 kB
MemAvailable:    2145678 kB
Buffers:          123456 kB
`

const thermalFixture = "52314\n"
const uptimeFixture = "12345.67 9876.54\n"

func TestSystemCollector(t *testing.T) {
	dir := t.TempDir()
	statPath := filepath.Join(dir, "stat")
	meminfoPath := filepath.Join(dir, "meminfo")
	thermalPath := filepath.Join(dir, "temp")
	uptimePath := filepath.Join(dir, "uptime")

	writeFixtures := func(statContent string) {
		t.Helper()
		for _, f := range []struct {
			path    string
			content string
		}{
			{statPath, statContent},
			{meminfoPath, meminfoFixture},
			{thermalPath, thermalFixture},
			{uptimePath, uptimeFixture},
		} {
			if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	st := state.New()
	sc := NewSystem(SystemOpts{
		StatPath:    statPath,
		MeminfoPath: meminfoPath,
		ThermalPath: thermalPath,
		UptimePath:  uptimePath,
		State:       st,
	})

	ctx := context.Background()

	// First run: CPU percentages should be zero (no previous sample).
	writeFixtures(statFixture1)
	if err := sc.Run(ctx); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	snap, updated := st.SnapshotSystem()
	if updated.IsZero() {
		t.Fatal("system updated_at is zero after first run")
	}
	if snap.CPU.PercentUser != 0 {
		t.Errorf("first run PercentUser = %f, want 0", snap.CPU.PercentUser)
	}
	if snap.CPU.PercentSystem != 0 {
		t.Errorf("first run PercentSystem = %f, want 0", snap.CPU.PercentSystem)
	}

	// Verify non-CPU fields.
	wantTemp := 52.314
	if math.Abs(snap.TemperatureC-wantTemp) > 0.001 {
		t.Errorf("TemperatureC = %f, want %f", snap.TemperatureC, wantTemp)
	}
	wantUptime := 12345.67
	if math.Abs(snap.UptimeSeconds-wantUptime) > 0.001 {
		t.Errorf("UptimeSeconds = %f, want %f", snap.UptimeSeconds, wantUptime)
	}
	wantTotal := uint64(3884292) * 1024
	if snap.Memory.TotalBytes != wantTotal {
		t.Errorf("Memory.TotalBytes = %d, want %d", snap.Memory.TotalBytes, wantTotal)
	}
	wantAvail := uint64(2145678) * 1024
	if snap.Memory.AvailableBytes != wantAvail {
		t.Errorf("Memory.AvailableBytes = %d, want %d", snap.Memory.AvailableBytes, wantAvail)
	}
	wantUsed := wantTotal - wantAvail
	if snap.Memory.UsedBytes != wantUsed {
		t.Errorf("Memory.UsedBytes = %d, want %d", snap.Memory.UsedBytes, wantUsed)
	}
	if snap.Memory.PercentUsed == 0 {
		t.Error("Memory.PercentUsed should be > 0")
	}
	if snap.BootTime.Unix() != 1680012345 {
		t.Errorf("BootTime = %v, want Unix 1680012345", snap.BootTime)
	}

	// Second run with different CPU counters: should compute delta percentages.
	writeFixtures(statFixture2)
	if err := sc.Run(ctx); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	snap, _ = st.SnapshotSystem()

	// Delta from fixture1 to fixture2:
	//   User: 10500-10000=500, Nice: 210-200=10, System: 3200-3000=200,
	//   Idle: 46800-46000=800, IOWait: 810-800=10
	//   Total = 500+10+200+800+10 = 1520
	//   PercentUser = (500+10)/1520*100 = 33.55...
	//   PercentSystem = 200/1520*100 = 13.15...
	//   PercentIdle = 800/1520*100 = 52.63...
	//   PercentIOWait = 10/1520*100 = 0.657...
	wantUser := float64(500+10) / 1520 * 100
	wantSys := float64(200) / 1520 * 100
	wantIdle := float64(800) / 1520 * 100
	wantIO := float64(10) / 1520 * 100

	if math.Abs(snap.CPU.PercentUser-wantUser) > 0.01 {
		t.Errorf("PercentUser = %f, want ~%f", snap.CPU.PercentUser, wantUser)
	}
	if math.Abs(snap.CPU.PercentSystem-wantSys) > 0.01 {
		t.Errorf("PercentSystem = %f, want ~%f", snap.CPU.PercentSystem, wantSys)
	}
	if math.Abs(snap.CPU.PercentIdle-wantIdle) > 0.01 {
		t.Errorf("PercentIdle = %f, want ~%f", snap.CPU.PercentIdle, wantIdle)
	}
	if math.Abs(snap.CPU.PercentIOWait-wantIO) > 0.01 {
		t.Errorf("PercentIOWait = %f, want ~%f", snap.CPU.PercentIOWait, wantIO)
	}
}

func TestSystemPreservesMediumTierFields(t *testing.T) {
	dir := t.TempDir()
	statPath := filepath.Join(dir, "stat")
	meminfoPath := filepath.Join(dir, "meminfo")
	thermalPath := filepath.Join(dir, "temp")
	uptimePath := filepath.Join(dir, "uptime")

	for _, f := range []struct {
		path    string
		content string
	}{
		{statPath, statFixture1},
		{meminfoPath, meminfoFixture},
		{thermalPath, thermalFixture},
		{uptimePath, uptimeFixture},
	} {
		if err := os.WriteFile(f.path, []byte(f.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	st := state.New()

	// Pre-populate medium-tier fields via SetSystem.
	st.SetSystem(model.SystemStats{
		Throttled:     "0x0",
		ThrottledFlag: false,
		Services: []model.ServiceState{
			{Name: "nftables", Active: true, RawState: "active"},
		},
	})

	sc := NewSystem(SystemOpts{
		StatPath:    statPath,
		MeminfoPath: meminfoPath,
		ThermalPath: thermalPath,
		UptimePath:  uptimePath,
		State:       st,
	})

	if err := sc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	snap, _ := st.SnapshotSystem()

	if snap.Throttled != "0x0" {
		t.Errorf("Throttled = %q, want %q (preserved)", snap.Throttled, "0x0")
	}
	if len(snap.Services) != 1 || snap.Services[0].Name != "nftables" {
		t.Errorf("Services not preserved: %v", snap.Services)
	}
}
