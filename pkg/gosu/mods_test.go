package gosu

import "testing"

// IsRanked is the surviving osu rule: which mods keep a score pp-eligible. Unranked
// mods, and a customized otherwise-ranked mod, disqualify it.
func TestMods_IsRanked(t *testing.T) {
	for _, acronym := range []string{"RX", "AP", "DA", "DC", "AS"} {
		if (Mods{{Acronym: acronym}}).IsRanked() {
			t.Errorf("IsRanked() = true with unranked mod %q, want false", acronym)
		}
	}
	if !(Mods{{Acronym: "HD"}, {Acronym: "SO"}}).IsRanked() {
		t.Error("IsRanked() = false with ranked mods HD,SO, want true")
	}
	tuned := Mods{{Acronym: "DT", Settings: map[string]any{"speed_change": 1.4}}}
	if tuned.IsRanked() {
		t.Error("IsRanked() = true for a customized Double Time, want false")
	}
}
