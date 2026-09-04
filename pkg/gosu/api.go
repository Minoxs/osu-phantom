package gosu

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// maxBulkIDs is the id ceiling osu! enforces on its bulk users and beatmaps
// endpoints. A request over it is rejected before any call is made.
const maxBulkIDs = 50

// ErrTooManyIDs is returned by the bulk fetches when more ids are passed than osu!
// accepts in one request.
var ErrTooManyIDs = fmt.Errorf("at most %d ids per bulk request", maxBulkIDs)

// bulkIDsURL builds an endpoint that repeats the ids[] query parameter, the form
// osu!'s bulk users and beatmaps endpoints read a list of ids from.
func bulkIDsURL(endpoint string, ids []int64) string {
	q := url.Values{}
	for _, id := range ids {
		q.Add("ids[]", strconv.FormatInt(id, 10))
	}
	return APIv2URL(endpoint + "?" + q.Encode())
}

type Credentials struct {
	ClientID     int
	ClientSecret string
}

// APIVersion is the osu! API v2 version this package requests, sent as the
// x-api-version header on every v2 call. osu-web gates response-shape changes on
// it and compares it as an integer, so pinning the newest published version keeps
// responses on the current solo_score shape. Update this when osu! ships a newer
// version whose shape this package has been adjusted to decode.
const APIVersion = "20241024"

// apiVersionHeader is the header name osu-web reads the version from.
const apiVersionHeader = "x-api-version"

func buildOAUTHUrl(endpoint string) string {
	return fmt.Sprintf("%s/%s", OAuthURL, endpoint)
}

func APIv2URL(endpoint string) string {
	return fmt.Sprintf("%s/%s", ApiV2, endpoint)
}

// GetUserID resolves a username to its user id or ErrUserNotFound
func (c *Client) GetUserID(username string) (int64, error) {
	endpoint := fmt.Sprintf("users/%s/osu/?key=username", username)

	req, _ := http.NewRequest(http.MethodGet, APIv2URL(endpoint), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return 0, ErrUserNotFound
	}
	if res.StatusCode != 200 {
		return 0, &StatusError{Code: res.StatusCode, Status: res.Status}
	}

	body := &User{}
	if err := json.NewDecoder(res.Body).Decode(body); err != nil {
		return 0, err
	}
	return body.ID, nil
}

func decodeUserExtended(res *http.Response) (*UserExtended, error) {
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, ErrUserNotFound
	}
	if res.StatusCode != 200 {
		return nil, &StatusError{Code: res.StatusCode, Status: res.Status}
	}

	user := &UserExtended{}
	if err := json.NewDecoder(res.Body).Decode(user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetUser fetches a profile by user id or ErrUserNotFound
func (c *Client) GetUser(id int64) (*UserExtended, error) {
	req, _ := http.NewRequest(http.MethodGet, APIv2URL(fmt.Sprintf("users/%d/osu", id)), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	return decodeUserExtended(res)
}

// GetUserByName fetches a profile by username or ErrUserNotFound
func (c *Client) GetUserByName(username string) (*UserExtended, error) {
	req, _ := http.NewRequest(http.MethodGet, APIv2URL(fmt.Sprintf("users/%s/osu?key=username", username)), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	return decodeUserExtended(res)
}

// GetRecentScores fetches one page of a user's recent scores newest first
func (c *Client) GetRecentScores(userid int64, limit, offset int) (FullScores, error) {
	endpoint := fmt.Sprintf("users/%d/scores/recent/?mode=osu&limit=%d&offset=%d", userid, limit, offset)

	req, _ := http.NewRequest(http.MethodGet, APIv2URL(endpoint), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, &StatusError{Code: res.StatusCode, Status: res.Status}
	}

	scores := make(FullScores, 0)
	if err := json.NewDecoder(res.Body).Decode(&scores); err != nil {
		return nil, err
	}

	return scores, nil
}

// GetScores fetches one page of the ruleset's global scores feed by cursor and returns the next cursor
// Scores carry a beatmap_id but no embedded beatmap
func (c *Client) GetScores(ruleset, cursor string) (Scores, string, error) {
	endpoint := "scores?ruleset=" + ruleset
	if cursor != "" {
		endpoint += "&cursor_string=" + cursor
	}

	req, _ := http.NewRequest(http.MethodGet, APIv2URL(endpoint), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, "", &StatusError{Code: res.StatusCode, Status: res.Status}
	}

	page := struct {
		Scores       Scores `json:"scores"`
		CursorString string `json:"cursor_string"`
	}{}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		return nil, "", err
	}
	return page.Scores, page.CursorString, nil
}

// GetBeatmapScores fetches all of a user's scores on one beatmap
func (c *Client) GetBeatmapScores(userID int64, beatmapID int64) (FullScores, error) {
	endpoint := fmt.Sprintf("beatmaps/%d/scores/users/%d/all", beatmapID, userID)

	req, _ := http.NewRequest(http.MethodGet, APIv2URL(endpoint), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, &StatusError{Code: res.StatusCode, Status: res.Status}
	}

	s := struct {
		Scores FullScores `json:"scores"`
	}{}
	if err := json.NewDecoder(res.Body).Decode(&s); err != nil {
		return nil, err
	}

	return s.Scores, nil
}

// GetBeatmap fetches a beatmap and its owning beatmapset by id
// Its max_combo is real unlike a score's embedded beatmap
func (c *Client) GetBeatmap(id int64) (BeatmapExtended, Beatmapset, error) {
	req, _ := http.NewRequest(http.MethodGet, APIv2URL(fmt.Sprintf("beatmaps/%d", id)), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return BeatmapExtended{}, Beatmapset{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return BeatmapExtended{}, Beatmapset{}, &StatusError{Code: res.StatusCode, Status: res.Status}
	}

	body := FullBeatmap{}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return BeatmapExtended{}, Beatmapset{}, err
	}
	return body.BeatmapExtended, body.Beatmapset, nil
}

// GetUsers fetches profiles for many ids in one request keyed by id in any order
// Returns ErrTooManyIDs above maxBulkIDs
// The bulk endpoint omits country rank so CountryRank and Rank stay nil
func (c *Client) GetUsers(ids []int64) ([]*User, error) {
	if len(ids) > maxBulkIDs {
		return nil, ErrTooManyIDs
	}
	if len(ids) == 0 {
		return nil, nil
	}

	req, _ := http.NewRequest(http.MethodGet, bulkIDsURL("users", ids), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, &StatusError{Code: res.StatusCode, Status: res.Status}
	}

	page := struct {
		Users []struct {
			User
			StatisticsRulesets struct {
				Osu UserStatistics `json:"osu"`
			} `json:"statistics_rulesets"`
		} `json:"users"`
	}{}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		return nil, err
	}

	profiles := make([]*User, 0, len(page.Users))
	for i := range page.Users {
		p := page.Users[i].User
		if p.Statistics == (UserStatistics{}) {
			p.Statistics = page.Users[i].StatisticsRulesets.Osu
		}
		profiles = append(profiles, &p)
	}
	return profiles, nil
}

// GetBeatmaps fetches metadata for many beatmap ids in one request keyed by id in any order
// Returns ErrTooManyIDs above maxBulkIDs
func (c *Client) GetBeatmaps(ids []int64) ([]FullBeatmap, error) {
	if len(ids) > maxBulkIDs {
		return nil, ErrTooManyIDs
	}
	if len(ids) == 0 {
		return nil, nil
	}

	req, _ := http.NewRequest(http.MethodGet, bulkIDsURL("beatmaps", ids), nil)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, &StatusError{Code: res.StatusCode, Status: res.Status}
	}

	page := struct {
		Beatmaps []FullBeatmap `json:"beatmaps"`
	}{}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		return nil, err
	}
	return page.Beatmaps, nil
}
