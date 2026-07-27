package game

// PlayerColorIDs is the curated set of cosmos-themed color choices players
// can pick when joining — deliberately a short, fixed list rather than a
// free color picker, so every choice reads clearly against the dark
// theme and stays visually distinct from the app's own semantic colors
// (correct/incorrect/recap). The frontend owns the matching hex values;
// the backend only needs to validate and store the chosen ID.
var PlayerColorIDs = []string{"nebula", "comet", "nova", "quasar", "solar", "crimson"}

// DefaultPlayerColor is used when a join request omits color, or names
// one outside PlayerColorIDs (e.g. an older client, or a stale palette).
const DefaultPlayerColor = "nebula"

// NormalizePlayerColor returns color if it's one of PlayerColorIDs, or
// DefaultPlayerColor otherwise. It never rejects a join over this —
// color is cosmetic, not worth failing a request over.
func NormalizePlayerColor(color string) string {
	for _, c := range PlayerColorIDs {
		if c == color {
			return color
		}
	}
	return DefaultPlayerColor
}
