package proc

import (
	"os"
	"path/filepath"
	"testing"
)

const testStat = `cpu  10132153 290696 3084719 46828483 16683 0 25195 0 0 0
cpu0 1393280 32966 572056 13343292 6130 0 17875 0 0 0
cpu1 1335498 35507 548183 13287498 3560 0 3152 0 0 0
cpu2 3443359 110431 990858 10111748 3580 0 2311 0 0 0
cpu3 3960016 111792 973622 10086045 3413 0 1857 0 0 0
intr 199292223 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
ctxt 38014093
btime 1680012345
processes 26442
procs_running 1
procs_blocked 0
softirq 5057579 250191 1481983 4 54 195590 0 2520 1146534 0 980703
`

func TestReadStat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stat")
	if err := os.WriteFile(path, []byte(testStat), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := ReadStat(path)
	if err != nil {
		t.Fatal(err)
	}

	if s.CPU.User != 10132153 {
		t.Errorf("User = %d, want 10132153", s.CPU.User)
	}
	if s.CPU.Nice != 290696 {
		t.Errorf("Nice = %d, want 290696", s.CPU.Nice)
	}
	if s.CPU.System != 3084719 {
		t.Errorf("System = %d, want 3084719", s.CPU.System)
	}
	if s.CPU.Idle != 46828483 {
		t.Errorf("Idle = %d, want 46828483", s.CPU.Idle)
	}
	if s.CPU.IOWait != 16683 {
		t.Errorf("IOWait = %d, want 16683", s.CPU.IOWait)
	}
	if s.BootTimeUnix != 1680012345 {
		t.Errorf("BootTimeUnix = %d, want 1680012345", s.BootTimeUnix)
	}
}

func TestReadStat_Total(t *testing.T) {
	c := CPUTimes{User: 100, Nice: 20, System: 30, Idle: 800, IOWait: 50}
	if c.Total() != 1000 {
		t.Errorf("Total() = %d, want 1000", c.Total())
	}
}

func TestReadStat_Delta(t *testing.T) {
	prev := CPUTimes{User: 100, Nice: 10, System: 50, Idle: 500, IOWait: 20}
	curr := CPUTimes{User: 200, Nice: 15, System: 80, Idle: 700, IOWait: 25}
	d := curr.Delta(prev)

	if d.User != 100 {
		t.Errorf("Delta User = %d, want 100", d.User)
	}
	if d.Nice != 5 {
		t.Errorf("Delta Nice = %d, want 5", d.Nice)
	}
	if d.System != 30 {
		t.Errorf("Delta System = %d, want 30", d.System)
	}
	if d.Idle != 200 {
		t.Errorf("Delta Idle = %d, want 200", d.Idle)
	}
	if d.IOWait != 5 {
		t.Errorf("Delta IOWait = %d, want 5", d.IOWait)
	}
	if d.Total() != 340 {
		t.Errorf("Delta Total() = %d, want 340", d.Total())
	}
}

func TestReadStat_MissingFile(t *testing.T) {
	_, err := ReadStat("/nonexistent/proc/stat")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadStat_NoCPULine(t *testing.T) {
	content := "btime 1680012345\n"
	path := filepath.Join(t.TempDir(), "stat_nocpu")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadStat(path)
	if err == nil {
		t.Fatal("expected error for missing cpu line")
	}
}

func TestReadStat_NoBtime(t *testing.T) {
	content := "cpu  100 20 30 800 50 0 0 0 0 0\n"
	path := filepath.Join(t.TempDir(), "stat_nobtime")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadStat(path)
	if err == nil {
		t.Fatal("expected error for missing btime")
	}
}
