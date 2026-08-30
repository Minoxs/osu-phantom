package gosu

import "fmt"

// Statistics is the lazer hit tally of a score. In osu!standard great/ok/meh are
// the 300/100/50 judgements.
type Statistics struct {
	Great int `json:"great"`
	Ok    int `json:"ok"`
	Meh   int `json:"meh"`
	Miss  int `json:"miss"`
}

func (s *Statistics) String() string {
	return fmt.Sprintf("great=%d : ok=%d : meh=%d : miss=%d", s.Great, s.Ok, s.Meh, s.Miss)
}
