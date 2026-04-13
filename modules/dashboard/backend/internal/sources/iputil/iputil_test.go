package iputil

import "testing"

func TestIsRFC1918(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},
		{"172.15.255.255", false},
		{"192.168.1.1", true},
		{"192.168.255.255", true},
		{"192.169.0.1", false},
		{"8.8.8.8", false},
		{"169.254.0.1", false},
		{"fe80::1", false},
		{"", false},
		{"notanip", false},
	}
	for _, tc := range cases {
		if got := IsRFC1918(tc.ip); got != tc.want {
			t.Errorf("IsRFC1918(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}
