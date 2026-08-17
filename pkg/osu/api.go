package osu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/minoxs/osu-phantom/pkg/osu/optimization"
	"github.com/minoxs/osu-phantom/pkg/osu/player"
	"io"
	"log/slog"
	"net/http"
)

type Credentials struct {
	ClientID     int
	ClientSecret string
}

func buildOAUTHUrl(endpoint string) string {
	return fmt.Sprintf("%s/%s", OAuthURL, endpoint)
}

func APIv2URL(endpoint string) string {
	return fmt.Sprintf("%s/%s", ApiV2, endpoint)
}

func createRequestBody(i interface{}) io.Reader {
	data, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return bytes.NewBuffer(data)
}

func GetGuestToken(c Credentials) (*GuestToken, error) {
	body := AuthGrant{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		GrantType:    "client_credentials",
		Scope:        "public",
	}
	r, err := apiClient.Post(buildOAUTHUrl("token"), JSON, createRequestBody(body))

	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode == 200 {
		res := &GuestToken{}
		return res, json.NewDecoder(r.Body).Decode(res)
	} else {
		return nil, errors.New("status_code=" + r.Status)
	}
}

func GetUserID(token *GuestToken, username string) (int, error) {
	endpoint := fmt.Sprintf("users/%s/osu/?key=username", username)

	req, _ := http.NewRequest(GET, APIv2URL(endpoint), nil)
	req.Header.Add(AUTH, createHeader(token.TokenType, token.AccessToken))

	res, err := apiClient.Do(req)
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

func decodeProfile(res *http.Response) (*player.Profile, error) {
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, ErrUserNotFound
	}
	if res.StatusCode != 200 {
		return nil, errors.New("status_code=" + res.Status)
	}

	profile := &player.Profile{}
	if err := json.NewDecoder(res.Body).Decode(profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// GetUser fetches a full osu!standard profile by user id.
// Returns ErrUserNotFound when no user carries that id.
func GetUser(token *GuestToken, id int64) (*player.Profile, error) {
	req, _ := http.NewRequest(GET, APIv2URL(fmt.Sprintf("users/%d/osu", id)), nil)
	req.Header.Add(AUTH, createHeader(token.TokenType, token.AccessToken))

	res, err := apiClient.Do(req)
	if err != nil {
		return nil, err
	}
	return decodeProfile(res)
}

// GetUserByName fetches a full osu!standard profile by username.
// Returns ErrUserNotFound when no user carries that name.
func GetUserByName(token *GuestToken, username string) (*player.Profile, error) {
	req, _ := http.NewRequest(GET, APIv2URL(fmt.Sprintf("users/%s/osu?key=username", username)), nil)
	req.Header.Add(AUTH, createHeader(token.TokenType, token.AccessToken))

	res, err := apiClient.Do(req)
	if err != nil {
		return nil, err
	}
	return decodeProfile(res)
}

// GetRecentScores fetches one page of a user's recent osu!standard scores,
// newest first. limit is capped at 100 by the osu! API; offset pages past the
// newest results.
func GetRecentScores(token *GuestToken, userid, limit, offset int) player.Scores {
	endpoint := fmt.Sprintf("users/%d/scores/recent/?mode=osu&limit=%d&offset=%d", userid, limit, offset)

	req, _ := http.NewRequest(GET, APIv2URL(endpoint), nil)
	req.Header.Add(AUTH, createHeader(token.TokenType, token.AccessToken))

	res, err := apiClient.Do(req)
	if err != nil {
		slog.Error("Error while sending request", "Error", err)
		return nil
	}
	defer res.Body.Close()

	scores := make(player.Scores, 0)
	err = json.NewDecoder(res.Body).Decode(&scores)
	if err != nil {
		slog.Error("Error while decoding response", "Error", err)
		return nil
	}

	return scores
}

func GetBeatmapScores(token *GuestToken, userID int, beatmapID int) player.Scores {
	endpoint := fmt.Sprintf("beatmaps/%d/scores/users/%d/all", beatmapID, userID)

	req, _ := http.NewRequest(GET, APIv2URL(endpoint), nil)
	req.Header.Add(AUTH, createHeader(token.TokenType, token.AccessToken))

	res, err := apiClient.Do(req)
	if err != nil {
		slog.Error("Error while sending request", "Error", err)
		return nil
	}
	defer res.Body.Close()

	s := struct{ Scores player.Scores }{make(player.Scores, 0)}
	err = json.NewDecoder(res.Body).Decode(&s)
	if err != nil {
		slog.Error("Error while decoding response", "Error", err)
		return nil
	}

	return s.Scores
}

// beatmapMeta bundles the display details a streamed score is missing.
type beatmapMeta struct {
	beatmap    player.Beatmap
	beatmapSet player.BeatmapSet
}

// beatmapMetaCache memoizes beatmap metadata so a map's display details are
// fetched from the osu! API at most once, process-wide, and reused across users.
var beatmapMetaCache = optimization.NewCache[int64, beatmapMeta](10000)

// beatmapResponse decodes GET /beatmaps/{id}: the difficulty fields sit at the
// top level with the parent set nested under "beatmapset".
type beatmapResponse struct {
	player.Beatmap
	BeatmapSet player.BeatmapSet `json:"beatmapset"`
}

// GetBeatmapMeta fetches a beatmap's display metadata (its difficulty and parent
// set) by id, serving repeat lookups from a process-wide cache. Streamed scores
// carry only a beatmap id, so this fills in the title, artist, difficulty and
// covers a profile needs without re-requesting a map already seen.
func GetBeatmapMeta(token *GuestToken, id int64) (player.Beatmap, player.BeatmapSet, error) {
	if meta, found := beatmapMetaCache.Get(id); found {
		return meta.beatmap, meta.beatmapSet, nil
	}

	req, _ := http.NewRequest(GET, APIv2URL(fmt.Sprintf("beatmaps/%d", id)), nil)
	req.Header.Add(AUTH, createHeader(token.TokenType, token.AccessToken))

	res, err := apiClient.Do(req)
	if err != nil {
		return player.Beatmap{}, player.BeatmapSet{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return player.Beatmap{}, player.BeatmapSet{}, errors.New("status_code=" + res.Status)
	}

	var body beatmapResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return player.Beatmap{}, player.BeatmapSet{}, err
	}

	beatmapMetaCache.Set(id, beatmapMeta{beatmap: body.Beatmap, beatmapSet: body.BeatmapSet})
	return body.Beatmap, body.BeatmapSet, nil
}

func DownloadBeatmap(id int64) (buf []byte, err error) {
	if beatmap, found := optimization.GetBeatmap(id); found {
		return beatmap, nil
	}

	var url = BaseURL + "/osu/" + fmt.Sprintf("%d", id)
	var res *http.Response

	res, err = apiClient.Get(url)
	if err != nil {
		return
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		err = errors.New("status_code=" + res.Status)
		return
	}

	buf, err = io.ReadAll(res.Body)
	slog.Info("Beatmap downloaded", "ID", id, "Size", len(buf))
	if err == nil {
		optimization.PutBeatmap(id, buf)
	}

	return
}
