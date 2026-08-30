package gosu

// Mod is one mod on a score in the osu! API v2 shape. The API emits Settings only
// when the mod is customized, which itself unranks an otherwise ranked mod.
type Mod struct {
	Acronym  string         `json:"acronym"`
	Settings map[string]any `json:"settings,omitempty"`
}

// Mods is a score's mod list in the osu! API v2 shape, an array of objects.
type Mods []Mod

// Acronyms returns the mod acronyms in order, e.g. ["HD","DT"].
func (m Mods) Acronyms() []string {
	out := make([]string, len(m))
	for i := range m {
		out[i] = m[i].Acronym
	}
	return out
}

// rankedAcronym reports whether an osu!standard mod earns pp at its default settings.
func rankedAcronym(acronym string) bool {
	switch acronym {
	case "EZ", // Easy
		"NF", // No Fail
		"HT", // Half Time
		"HR", // Hard Rock
		"SD", // Sudden Death
		"PF", // Perfect
		"DT", // Double Time
		"NC", // Nightcore
		"HD", // Hidden
		"FL", // Flashlight
		"SO", // Spun Out
		"CL", // Classic
		"TD": // Touch Device
		return true
	default:
		return false
	}
}

// IsRanked reports whether every mod keeps the score pp-eligible: a ranked acronym
// carrying no custom settings, since customizing a mod unranks the score.
func (m Mods) IsRanked() bool {
	for i := range m {
		if !rankedAcronym(m[i].Acronym) {
			return false
		}
		if len(m[i].Settings) > 0 {
			return false
		}
	}
	return true
}
