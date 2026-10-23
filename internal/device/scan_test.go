package device

import "testing"

func TestParseSubnet(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"192.168.10.0", "192.168.10", true},
		{"192.168.10.0/24", "192.168.10", true},
		{" 192.168.10 ", "192.168.10", true},
		{"10.0.0.0", "10.0.0", true},
		{"192.168.10.5", "", false},
		{"192.168.10.0/16", "", false},
		{"192.168.300.0", "", false},
		{"192.168", "", false},
		{"", "", false},
		{"bike.local", "", false},
	}
	for _, c := range cases {
		got, ok := ParseSubnet(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseSubnet(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}
