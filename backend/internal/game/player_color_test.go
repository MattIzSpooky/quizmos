package game

import "testing"

func TestNormalizePlayerColor_AcceptsKnownColors(t *testing.T) {
	for _, c := range PlayerColorIDs {
		if got := NormalizePlayerColor(c); got != c {
			t.Errorf("NormalizePlayerColor(%q) = %q, want %q", c, got, c)
		}
	}
}

func TestNormalizePlayerColor_FallsBackForUnknownOrEmpty(t *testing.T) {
	cases := []string{"", "not-a-color", "NEBULA", "comet "}
	for _, c := range cases {
		if got := NormalizePlayerColor(c); got != DefaultPlayerColor {
			t.Errorf("NormalizePlayerColor(%q) = %q, want default %q", c, got, DefaultPlayerColor)
		}
	}
}
