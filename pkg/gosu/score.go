package gosu

import (
	"fmt"
	"time"
)

// BeatmapStatus is the osu! ranking status of a map, as the API reports it.
type BeatmapStatus string

const (
	StatusGraveyard BeatmapStatus = "graveyard"
	StatusWIP       BeatmapStatus = "wip"
	StatusPending   BeatmapStatus = "pending"
	StatusRanked    BeatmapStatus = "ranked"
	StatusApproved  BeatmapStatus = "approved"
	StatusQualified BeatmapStatus = "qualified"
	StatusLoved     BeatmapStatus = "loved"
)

// AwardsPP reports whether scores on a map of this status earn pp. osu! awards it
// only on ranked and approved maps; loved, qualified, and unsubmitted give none.
func (s BeatmapStatus) AwardsPP() bool {
	return s == StatusRanked || s == StatusApproved
}

type Beatmap struct {
	ID            int64         `json:"id"`
	Version       string        `json:"version"`
	StarRating    float32       `json:"difficulty_rating"`
	Status        BeatmapStatus `json:"status"`
	Mode          string        `json:"mode"`
	MaxCombo      int           `json:"max_combo"`
	TotalLength   int           `json:"total_length"`
	OD            float32       `json:"accuracy"`
	Ar            float32       `json:"ar"`
	CountCircles  int           `json:"count_circles"`
	CountSliders  int           `json:"count_sliders"`
	CountSpinners int           `json:"count_spinners"`
	CS            float32       `json:"cs"`
	HP            float32       `json:"drain"`
	HitLength     int           `json:"hit_length"`
}

// Covers holds the image URLs the osu API attaches to a beatmapset.
type Covers struct {
	Cover string `json:"cover"`
	Card  string `json:"card"`
	List  string `json:"list"`
	Slim  string `json:"slimcover"`
}

type BeatmapSet struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Creator string `json:"creator"`
	Covers  Covers `json:"covers"`
}

// FullBeatmap is a beatmap with its owning set embedded, as the single-beatmap and
// bulk-beatmaps endpoints return it: the map for its status and difficulty, the set
// for its title, artist, and cover art.
type FullBeatmap struct {
	Beatmap
	BeatmapSet BeatmapSet `json:"beatmapset"`
}

// Score is an osu! API v2 "solo_score": one play as osu! stores it under the
// lazer submission pipeline. Requests must send a modern x-api-version to receive
// this shape. It is the lean form the global scores feed returns, carrying only a
// beatmap_id and no embedded beatmap or beatmapset. Use FullScore where the API
// embeds those, such as the user-scores endpoints.
type Score struct {
	ID         int64     `json:"id"`
	UserID     int       `json:"user_id"`
	BeatmapID  int64     `json:"beatmap_id"`
	RulesetID  int       `json:"ruleset_id"`
	EndedAt    time.Time `json:"ended_at"`
	Accuracy   float32   `json:"accuracy"`
	Mods       Mods      `json:"mods"`
	TotalScore int       `json:"total_score"`
	MaxCombo   int       `json:"max_combo"`
	Rank       string    `json:"rank"`
	Passed     bool      `json:"passed"`
	// Ranked is osu's beatmap-leaderboard-visibility flag. Notably it does not track
	// whether the score gives pp, so it is not a pp signal.
	Ranked bool `json:"ranked"`
	// Processed is false until osu finishes server-side processing, most notably the
	// pp calculation, so a false value means PP is not yet authoritative.
	Processed  bool       `json:"processed"`
	PP         float64    `json:"pp"`
	Statistics Statistics `json:"statistics"`
}

// FullScore is a Score as the endpoints that embed the map return it: a user's
// score list or a beatmap's score list carry both the beatmap and its set inline.
type FullScore struct {
	Score
	Beatmap    Beatmap    `json:"beatmap"`
	BeatmapSet BeatmapSet `json:"beatmapset"`
}

// Mode is the score's ruleset as a mode name (osu/taiko/fruits/mania). The
// solo_score wire reports the mode as a numeric ruleset id.
func (s Score) Mode() string {
	switch s.RulesetID {
	case 0:
		return "osu"
	case 1:
		return "taiko"
	case 2:
		return "fruits"
	case 3:
		return "mania"
	default:
		return ""
	}
}

type Scores []Score

// FullScores is a list of scores with their maps embedded, as the user-scores and
// beatmap-scores endpoints return.
type FullScores []FullScore

func (s Scores) String() (res string) {
	res = ""
	for _, score := range s {
		res += score.String() + ",\n"
	}

	if len(res) > 0 {
		return "[\n" + res[:len(res)-2] + "\n]"
	} else {
		return "[]"
	}
}

func (s *Score) String() string {
	return fmt.Sprintf(
		"{ ID=%d : BeatmapID=%d : Mode=%s : Mods=%v : Score=%d : PP=%.0f : MaxCombo=%d : Acc=%.2f : %s }",
		s.ID,
		s.BeatmapID,
		s.Mode(),
		s.Mods.Acronyms(),
		s.TotalScore,
		s.PP,
		s.MaxCombo,
		s.Accuracy,
		s.Statistics.String(),
	)
}
