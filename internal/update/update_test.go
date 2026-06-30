package update

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.1.0", "v0.1.1", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.2.0", "v1.1.9", false},
		{"0.1.0", "v0.1.0", false}, // missing "v" still compares equal
		{"dev", "v0.1.0", true},    // dev is always behind a real release
		{"v0.1.0", "", false},      // no known latest → never newer
		{"v0.1.0", "v0.1.1-rc1", true},
		{"v0.1.0", "garbage", false}, // unparseable latest → not newer
		{"v2.0.0", "v10.0.0", true},  // numeric, not lexical
	}
	for _, c := range cases {
		if got := isNewer(c.current, c.latest); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}
