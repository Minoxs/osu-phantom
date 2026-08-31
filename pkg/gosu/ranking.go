package gosu

import (
	"fmt"
	"math"
	"slices"
)

// RankSize caps how many scores a ranking keeps: osu! weights only the top 100.
const RankSize = 100

// Ranking holds a player's best scores for a window, at most one per beatmap,
// ordered by pp descending and capped at RankSize.
type Ranking struct {
	scores []Score
}

func (r *Ranking) String() string {
	return fmt.Sprintf("Count=%d : TotalPP=%.0f : Scores=%v", len(r.scores), r.GetTotalPP(), Scores(r.scores))
}

// findPosition locates where s belongs. valid is false when s should not be added:
// it is already present, or a better score on the same map already ranks. pos is
// the insertion index, and valid also requires it to fall within RankSize.
func (r *Ranking) findPosition(s *Score) (valid bool, pos int) {
	for pos = 0; pos < len(r.scores); pos++ {
		// Already added, or a better score on the same map already exists.
		if s.ID == r.scores[pos].ID || (s.BeatmapID == r.scores[pos].BeatmapID && s.PP <= r.scores[pos].PP) {
			return
		}
		// Better score.
		if s.PP > r.scores[pos].PP {
			break
		}
	}
	valid = pos < RankSize
	return
}

// insertScore places s at pos, dropping any lower-ranked score on the same map.
// findPosition guarantees that duplicate can only sit at or after pos, so removing
// it never shifts pos.
func (r *Ranking) insertScore(pos int, s *Score) {
	for j := pos; j < len(r.scores); j++ {
		if r.scores[j].BeatmapID == s.BeatmapID {
			r.scores = slices.Delete(r.scores, j, j+1)
			break
		}
	}

	r.scores = append(r.scores, Score{})
	copy(r.scores[pos+1:], r.scores[pos:])
	r.scores[pos] = *s

	if len(r.scores) > RankSize {
		r.scores = r.scores[:RankSize]
	}
}

func (r *Ranking) AddScore(s Score) (rank int, added bool) {
	valid, pos := r.findPosition(&s)
	if valid {
		r.insertScore(pos, &s)
	}
	return pos + 1, valid
}

// RemoveScore drops the score with the given id, so a consumer can retract a play
// from the window and let the rest re-rank. It reports whether a score was removed.
func (r *Ranking) RemoveScore(id int64) (removed bool) {
	for i := range r.scores {
		if r.scores[i].ID == id {
			r.scores = slices.Delete(r.scores, i, i+1)
			return true
		}
	}
	return false
}

func (r *Ranking) GetTotalPP() (res float64) {
	for i := range r.scores {
		res += r.scores[i].PP * math.Pow(0.95, float64(i))
	}
	return math.Floor(res)
}

// Scores returns an independent copy of the ranked scores, so a caller may read or
// mutate the result without touching the ranking's backing store.
func (r Ranking) Scores() Scores {
	out := make(Scores, len(r.scores))
	copy(out, r.scores)
	return out
}

func (r Ranking) Count() int {
	return len(r.scores)
}
