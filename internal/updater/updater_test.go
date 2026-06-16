package updater

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v2.0.0", "v1.0.2", true},
		{"v1.0.3", "v1.0.2", true},
		{"v1.1.0", "v1.0.9", true},
		{"v1.0.2", "v1.0.2", false},
		{"v1.0.1", "v1.0.2", false},
		{"v2.0.0", "dev", true},            // non-semver current -> update offered
		{"v2.0.0", "v1.0.2-10-gabc", true}, // describe-style suffix dropped
		{"v1.0.2", "v1.0.2-SNAPSHOT-ab", false},
		{"not-a-version", "v1.0.0", false}, // bad latest -> no update
	}
	for _, c := range cases {
		if got := IsNewer(c.latest, c.current); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	if got := parseSemver("v1.2.3"); got == nil || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("parseSemver(v1.2.3) = %v", got)
	}
	if got := parseSemver("1.2.3-SNAPSHOT-abc"); got == nil || got[2] != 3 {
		t.Errorf("parseSemver with suffix = %v", got)
	}
	if got := parseSemver("garbage"); got != nil {
		t.Errorf("parseSemver(garbage) = %v, want nil", got)
	}
}
