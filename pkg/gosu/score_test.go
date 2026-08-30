package gosu

import (
	"encoding/json"
	"testing"
	"time"
)

// sample is a trimmed osu! API v2 solo_score object from an endpoint that embeds
// the map, so it decodes into a FullScore. It keeps the fields the types decode and
// their real shapes (mods as objects, lazer statistics, ended_at, ruleset_id,
// processed, nested beatmap/beatmapset).
const sample = `{
  "id": 7027024157,
  "user_id": 30692023,
  "beatmap_id": 2335023,
  "ended_at": "2026-07-07T03:30:08Z",
  "accuracy": 0.91954,
  "mods": [{"acronym": "HD"}, {"acronym": "DT"}],
  "total_score": 700980,
  "max_combo": 95,
  "rank": "A",
  "passed": true,
  "ranked": true,
  "processed": true,
  "pp": 24.5157,
  "ruleset_id": 0,
  "statistics": {"great": 305, "ok": 30, "meh": 2, "miss": 13, "ignore_hit": 1},
  "beatmap": {"id": 2335023, "status": "ranked", "difficulty_rating": 6.2, "version": "Extra", "mode": "osu"},
  "beatmapset": {"id": 55, "title": "Song", "artist": "Artist", "creator": "Mapper"}
}`

func TestFullScoreDecode(t *testing.T) {
	var s FullScore
	if err := json.Unmarshal([]byte(sample), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s.ID != 7027024157 || s.UserID != 30692023 || s.BeatmapID != 2335023 {
		t.Errorf("id/user/beatmap = %d/%d/%d", s.ID, s.UserID, s.BeatmapID)
	}
	if want := time.Date(2026, 7, 7, 3, 30, 8, 0, time.UTC); !s.EndedAt.Equal(want) {
		t.Errorf("EndedAt = %v, want %v", s.EndedAt, want)
	}
	if got := s.Mods.Acronyms(); len(got) != 2 || got[0] != "HD" || got[1] != "DT" {
		t.Errorf("Mods = %v, want [HD DT]", got)
	}
	if s.Statistics.Great != 305 || s.Statistics.Ok != 30 || s.Statistics.Meh != 2 || s.Statistics.Miss != 13 {
		t.Errorf("statistics = %+v, want 305/30/2/13", s.Statistics)
	}
	if s.TotalScore != 700980 || s.Mode() != "osu" {
		t.Errorf("total/mode = %d/%q", s.TotalScore, s.Mode())
	}
	if s.PP != 24.5157 || !s.Passed || !s.Ranked || !s.Processed {
		t.Errorf("pp/passed/ranked/processed = %v/%v/%v/%v", s.PP, s.Passed, s.Ranked, s.Processed)
	}
	if s.Beatmap.ID != 2335023 || s.Beatmap.Status != StatusRanked {
		t.Errorf("beatmap = %d/%q", s.Beatmap.ID, s.Beatmap.Status)
	}
	if s.BeatmapSet.Title != "Song" || s.BeatmapSet.Creator != "Mapper" {
		t.Errorf("set = %q/%q", s.BeatmapSet.Title, s.BeatmapSet.Creator)
	}
}

func TestScoreMode(t *testing.T) {
	cases := map[int]string{0: "osu", 1: "taiko", 2: "fruits", 3: "mania", 9: ""}
	for id, want := range cases {
		if got := (Score{RulesetID: id}).Mode(); got != want {
			t.Errorf("ruleset %d Mode() = %q, want %q", id, got, want)
		}
	}
}
