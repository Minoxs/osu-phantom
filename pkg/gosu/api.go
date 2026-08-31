package gosu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

func createRequestBody(i any) io.Reader {
	data, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return bytes.NewBuffer(data)
}

func (c *Client) GetGuestToken(creds Credentials) (*GuestToken, error) {
	body := AuthGrant{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		GrantType:    "client_credentials",
		Scope:        "public",
	}
	req, err := http.NewRequest(http.MethodPost, buildOAUTHUrl("token"), createRequestBody(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mimeJSON)
	r, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		return nil, errors.New("status_code=" + r.Status)
	}

	res := &GuestToken{}
	return res, json.NewDecoder(r.Body).Decode(res)
}

func (c *Client) GetUserID(token *GuestToken, username string) (int64, error) {
	endpoint := fmt.Sprintf("users/%s/osu/?key=username", username)

	req, _ := http.NewRequest(http.MethodGet, APIv2URL(endpoint), nil)
	req.Header.Add(authHeader, createHeader(token.TokenType, token.AccessToken))
	req.Header.Add(apiVersionHeader, APIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	body := &User{}
	err = json.NewDecoder(res.Body).Decode(body)
	if err != nil {
		return 0, err
	}
	return body.ID, err
}

func decodeUserExtended(res *http.Response) (*UserExtended, error) {
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, ErrUserNotFound
	}
	if res.StatusCode != 200 {
		return nil, errors.New("status_code=" + res.Status)
	}

	user := &UserExtended{}
	if err := json.NewDecoder(res.Body).Decode(user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetUser fetches a full osu!standard profile by user id. Returns ErrUserNotFound
// when no user carries that id.
func (c *Client) GetUser(token *GuestToken, id int64) (*UserExtended, error) {
	req, _ := http.NewRequest(http.MethodGet, APIv2URL(fmt.Sprintf("users/%d/osu", id)), nil)
	req.Header.Add(authHeader, createHeader(token.TokenType, token.AccessToken))
	req.Header.Add(apiVersionHeader, APIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	return decodeUserExtended(res)
}

// GetUserByName fetches a full osu!standard profile by username. Returns
// ErrUserNotFound when no user carries that name.
func (c *Client) GetUserByName(token *GuestToken, username string) (*UserExtended, error) {
	req, _ := http.NewRequest(http.MethodGet, APIv2URL(fmt.Sprintf("users/%s/osu?key=username", username)), nil)
	req.Header.Add(authHeader, createHeader(token.TokenType, token.AccessToken))
	req.Header.Add(apiVersionHeader, APIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	return decodeUserExtended(res)
}

// GetRecentScores fetches one page of a user's recent osu!standard scores,
// newest first. limit is capped at 100 by the osu! API; offset pages past the
// newest results.
func (c *Client) GetRecentScores(token *GuestToken, userid int64, limit, offset int) FullScores {
	endpoint := fmt.Sprintf("users/%d/scores/recent/?mode=osu&limit=%d&offset=%d", userid, limit, offset)

	req, _ := http.NewRequest(http.MethodGet, APIv2URL(endpoint), nil)
	req.Header.Add(authHeader, createHeader(token.TokenType, token.AccessToken))
	req.Header.Add(apiVersionHeader, APIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		slog.Error("Error while sending request", "Error", err)
		return nil
	}
	defer res.Body.Close()

	scores := make(FullScores, 0)
	err = json.NewDecoder(res.Body).Decode(&scores)
	if err != nil {
		slog.Error("Error while decoding response", "Error", err)
		return nil
	}

	return scores
}

// GetScores fetches one page of osu!'s global recent-scores feed for a ruleset,
// the passing scores every player has set, ascending by id. cursor is the
// cursor_string from a previous page, or empty for the newest page; the returned
// cursor_string fetches the scores newer than this page. Scores carry no embedded
// beatmap, only a beatmap_id.
func (c *Client) GetScores(token *GuestToken, ruleset, cursor string) (Scores, string, error) {
	endpoint := "scores?ruleset=" + ruleset
	if cursor != "" {
		endpoint += "&cursor_string=" + cursor
	}

	req, _ := http.NewRequest(http.MethodGet, APIv2URL(endpoint), nil)
	req.Header.Add(authHeader, createHeader(token.TokenType, token.AccessToken))
	req.Header.Add(apiVersionHeader, APIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, "", errors.New("status_code=" + res.Status)
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

func (c *Client) GetBeatmapScores(token *GuestToken, userID int64, beatmapID int64) FullScores {
	endpoint := fmt.Sprintf("beatmaps/%d/scores/users/%d/all", beatmapID, userID)

	req, _ := http.NewRequest(http.MethodGet, APIv2URL(endpoint), nil)
	req.Header.Add(authHeader, createHeader(token.TokenType, token.AccessToken))
	req.Header.Add(apiVersionHeader, APIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		slog.Error("Error while sending request", "Error", err)
		return nil
	}
	defer res.Body.Close()

	s := struct {
		Scores FullScores `json:"scores"`
	}{}
	err = json.NewDecoder(res.Body).Decode(&s)
	if err != nil {
		slog.Error("Error while decoding response", "Error", err)
		return nil
	}

	return s.Scores
}

// GetBeatmap fetches a single beatmap's metadata by id. The osu! API nests the
// owning beatmapset in the response, so both are returned: the map for its status
// and difficulty, the set for its title, artist, and cover art. Unlike the beatmap
// embedded in a score, this response carries a real max_combo.
func (c *Client) GetBeatmap(token *GuestToken, id int64) (BeatmapExtended, Beatmapset, error) {
	req, _ := http.NewRequest(http.MethodGet, APIv2URL(fmt.Sprintf("beatmaps/%d", id)), nil)
	req.Header.Add(authHeader, createHeader(token.TokenType, token.AccessToken))
	req.Header.Add(apiVersionHeader, APIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		return BeatmapExtended{}, Beatmapset{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return BeatmapExtended{}, Beatmapset{}, errors.New("status_code=" + res.Status)
	}

	body := FullBeatmap{}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return BeatmapExtended{}, Beatmapset{}, err
	}
	return body.BeatmapExtended, body.Beatmapset, nil
}

// GetUsers fetches osu!standard profiles for many user ids in one request. It
// returns ErrTooManyIDs when more than maxBulkIDs ids are passed, and nil for an
// empty list without calling the API. osu! returns only the ids it finds, so the
// result may be shorter than ids and in any order; callers key it by profile id.
//
// The bulk endpoint reports ranking stats per ruleset, so the osu! stats are folded
// onto Statistics. It carries less than the single-user endpoint: it omits country
// rank entirely, so Statistics.CountryRank and Rank stay nil and a caller that needs
// country rank must fetch that user singly.
func (c *Client) GetUsers(token *GuestToken, ids []int64) ([]*User, error) {
	if len(ids) > maxBulkIDs {
		return nil, ErrTooManyIDs
	}
	if len(ids) == 0 {
		return nil, nil
	}

	req, _ := http.NewRequest(http.MethodGet, bulkIDsURL("users", ids), nil)
	req.Header.Add(authHeader, createHeader(token.TokenType, token.AccessToken))
	req.Header.Add(apiVersionHeader, APIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, errors.New("status_code=" + res.Status)
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

// GetBeatmaps fetches beatmap metadata for many ids in one request, each with its
// owning set embedded. It returns ErrTooManyIDs when more than maxBulkIDs ids are
// passed, and nil for an empty list without calling the API. osu! returns only the
// ids it finds, so the result may be shorter than ids and in any order; callers key
// it by beatmap id.
func (c *Client) GetBeatmaps(token *GuestToken, ids []int64) ([]FullBeatmap, error) {
	if len(ids) > maxBulkIDs {
		return nil, ErrTooManyIDs
	}
	if len(ids) == 0 {
		return nil, nil
	}

	req, _ := http.NewRequest(http.MethodGet, bulkIDsURL("beatmaps", ids), nil)
	req.Header.Add(authHeader, createHeader(token.TokenType, token.AccessToken))
	req.Header.Add(apiVersionHeader, APIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, errors.New("status_code=" + res.Status)
	}

	page := struct {
		Beatmaps []FullBeatmap `json:"beatmaps"`
	}{}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		return nil, err
	}
	return page.Beatmaps, nil
}
