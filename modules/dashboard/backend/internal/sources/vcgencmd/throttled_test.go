package vcgencmd

import (
	"testing"
)

func TestParseThrottled(t *testing.T) {
	raw, flag := parseThrottled("throttled=0x0")
	if raw != "0x0" {
		t.Errorf("raw = %q, want %q", raw, "0x0")
	}
	if flag {
		t.Error("flag should be false for 0x0")
	}

	raw, flag = parseThrottled("throttled=0x50000")
	if raw != "0x50000" {
		t.Errorf("raw = %q, want %q", raw, "0x50000")
	}
	if !flag {
		t.Error("flag should be true for 0x50000")
	}
}

func TestParseThrottledNoEquals(t *testing.T) {
	raw, flag := parseThrottled("garbage")
	if raw != "garbage" {
		t.Errorf("raw = %q, want %q", raw, "garbage")
	}
	if flag {
		t.Error("flag should be false for unparseable input")
	}
}
