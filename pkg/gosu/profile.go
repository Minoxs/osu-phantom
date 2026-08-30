package gosu

import "time"

// Ranks is osu!'s nested rank block, carrying the country rank the flat
// country_rank field duplicates on some responses.
type Ranks struct {
	Country *int `json:"country"`
}

// RankStatistics is a user's osu!standard ranking summary. The single-user endpoint
// returns it under statistics; the bulk users endpoint returns it per ruleset under
// statistics_rulesets, so it is named for reuse across both.
type RankStatistics struct {
	PP          float64 `json:"pp"`
	HitAccuracy float64 `json:"hit_accuracy"`
	PlayCount   int     `json:"play_count"`
	GlobalRank  *int    `json:"global_rank"`
	CountryRank *int    `json:"country_rank"`
	Rank        Ranks   `json:"rank"`
}

// Profile is a user's public osu! profile as returned by the users endpoint.
type Profile struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	CountryCode string    `json:"country_code"`
	AvatarURL   string    `json:"avatar_url"`
	CoverURL    string    `json:"cover_url"`
	IsSupporter bool      `json:"is_supporter"`
	IsOnline    bool      `json:"is_online"`
	JoinDate    time.Time `json:"join_date"`

	Country struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"country"`

	Cover struct {
		URL string `json:"url"`
	} `json:"cover"`

	Statistics RankStatistics `json:"statistics"`
}

// Cover image URL, preferring the nested cover object the API populates.
func (p *Profile) Banner() string {
	if p.Cover.URL != "" {
		return p.Cover.URL
	}
	return p.CoverURL
}

// GlobalRank is the all-time global rank, nil when the user is unranked.
func (p *Profile) GlobalRank() *int {
	return p.Statistics.GlobalRank
}

// CountryRank falls back to the legacy rank.country field when the flat one is absent.
func (p *Profile) CountryRank() *int {
	if p.Statistics.CountryRank != nil {
		return p.Statistics.CountryRank
	}
	return p.Statistics.Rank.Country
}

// Country2 is the ISO alpha-2 code, preferring the nested country object.
func (p *Profile) Country2() string {
	if p.Country.Code != "" {
		return p.Country.Code
	}
	return p.CountryCode
}
