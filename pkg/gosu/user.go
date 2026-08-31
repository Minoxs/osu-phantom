package gosu

import "time"

// Ranks is osu!'s nested rank block, carrying the country rank the flat
// country_rank field duplicates on some responses.
type Ranks struct {
	Country *int `json:"country"`
}

// UserStatistics is a user's osu!standard ranking summary. The single-user endpoint
// returns it under statistics; the bulk users endpoint returns it per ruleset under
// statistics_rulesets, so it is named for reuse across both.
type UserStatistics struct {
	PP          float64 `json:"pp"`
	HitAccuracy float64 `json:"hit_accuracy"`
	PlayCount   int     `json:"play_count"`
	GlobalRank  *int    `json:"global_rank"`
	CountryRank *int    `json:"country_rank"`
	Rank        Ranks   `json:"rank"`
}

// Country is an osu! user's country, present when the country attribute is included.
type Country struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Cover is an osu! user's profile cover, present when the cover attribute is included.
type Cover struct {
	URL string `json:"url"`
}

// User is osu!'s compact user object, returned embedded in other responses and by the
// bulk users endpoint. Country, Cover, and Statistics are optional attributes the API
// populates only when included or when the endpoint carries them.
type User struct {
	ID          int64          `json:"id"`
	Username    string         `json:"username"`
	CountryCode string         `json:"country_code"`
	AvatarURL   string         `json:"avatar_url"`
	CoverURL    string         `json:"cover_url"`
	IsSupporter bool           `json:"is_supporter"`
	IsOnline    bool           `json:"is_online"`
	Country     Country        `json:"country"`
	Cover       Cover          `json:"cover"`
	Statistics  UserStatistics `json:"statistics"`
}

// UserExtended is the full osu! profile the single-user endpoint returns: the compact
// User plus the fields that endpoint adds.
type UserExtended struct {
	User
	JoinDate time.Time `json:"join_date"`
}

// Banner is the cover image URL, preferring the nested cover object the API populates.
func (u *User) Banner() string {
	if u.Cover.URL != "" {
		return u.Cover.URL
	}
	return u.CoverURL
}

// GlobalRank is the all-time global rank, nil when the user is unranked.
func (u *User) GlobalRank() *int {
	return u.Statistics.GlobalRank
}

// CountryRank falls back to the legacy rank.country field when the flat one is absent.
func (u *User) CountryRank() *int {
	if u.Statistics.CountryRank != nil {
		return u.Statistics.CountryRank
	}
	return u.Statistics.Rank.Country
}

// Country2 is the ISO alpha-2 code, preferring the nested country object.
func (u *User) Country2() string {
	if u.Country.Code != "" {
		return u.Country.Code
	}
	return u.CountryCode
}
