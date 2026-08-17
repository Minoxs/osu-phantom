package phantom

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/minoxs/osu-phantom/pkg/osu"
	"github.com/minoxs/osu-phantom/pkg/osu/player"
)

type (
	// AuthProvider is required for requests which require OAuth authorization
	AuthProvider interface {
		GetToken() *osu.GuestToken
	}

	// NewScore contains information of a new score
	NewScore struct {
		Rank  int
		Score player.Score
	}

	// Client handles the tracking of a user's scores
	Client struct {
		UserID   int
		Username string
		Provider AuthProvider
		Logger   *slog.Logger

		OnNewScores func([]NewScore)
		ranking     player.Ranking
		LastUpdate  time.Time
		// WindowStart is the fixed instant tracking began. LastUpdate advances to
		// the newest score seen, but WindowStart never moves, so pushed scores are
		// judged against the opt-in moment rather than a sliding watermark.
		WindowStart time.Time

		lock sync.Mutex
	}
)

var ErrUserNotFound = errors.New("user not found")

// Login will look for the user and return a phantom client with the given user.
// Returns ErrUserNotFound if user is not found, or some HTTP error if failed to fetch user.
// Call Client.KeepUpdated to keep rankings constantly updated, or manually update with Client.Update.
func Login(provider AuthProvider, username string, start time.Time) (client *Client, err error) {
	client = &Client{Username: username, Provider: provider}
	client.UserID, err = osu.GetUserID(provider.GetToken(), client.Username)
	client.LastUpdate = start
	client.WindowStart = start
	client.Logger = slog.Default().With("Username", username)

	// API only returns error if request failed.
	// If user was not found, return ErrUserNotFound.
	if err == nil && client.UserID == 0 {
		err = ErrUserNotFound
	}

	// Unset client pointer if function errored.
	// This is only done to make the function fit go standards.
	if err != nil {
		client = nil
	}

	return
}

// NewClient builds a tracking client for a user whose id is already known,
// skipping the username lookup Login performs. Scores set after start are tracked.
func NewClient(provider AuthProvider, userID int, username string, start time.Time) *Client {
	return &Client{
		UserID:      userID,
		Username:    username,
		Provider:    provider,
		LastUpdate:  start,
		WindowStart: start,
		Logger:      slog.Default().With("Username", username),
	}
}

// Restore rebuilds the ranking from previously persisted scores and advances
// LastUpdate past the newest of them, so polling resumes without re-counting them.
func (c *Client) Restore(scores player.Scores) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for _, s := range scores {
		c.ranking.AddScore(s)
		if s.CreatedAt.After(c.LastUpdate) {
			c.LastUpdate = s.CreatedAt
		}
	}
}

// KeepUpdated will fetch new scores from the API in the interval configured.
// Will stop routine after maxIdle without new scores.
func (c *Client) KeepUpdated(checkInterval time.Duration, maxIdle time.Duration) {
	c.Logger.Info("Running KeepUpdated")
	defer func() {
		if r := recover(); r != nil {
			c.Logger.Error("Recovered from panic", "Panic", r)
		}
		c.Logger.Info("Stopping KeepUpdated routine")
	}()

	var interval = time.NewTimer(0)
	defer interval.Stop()

	for {
		select {
		case <-interval.C:
			c.Logger.Debug("Getting new scores")

			// Stop if idle for too long
			if !c.Update() && time.Now().Sub(c.LastUpdate) > maxIdle {
				return
			}

			// Reset timer
			interval.Reset(checkInterval)
		}
	}
}

// recentScorePageSize is the per-request score count. The osu! API caps it at 100.
const recentScorePageSize = 100

// Update checks for new scores and folds them into the ranking. It pages through
// recent scores by offset while every score on a page is newer than the previous
// watermark, so a burst larger than one page is not missed. Returns true when at
// least one newer score was seen.
func (c *Client) Update() bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	prev := c.LastUpdate
	var newest time.Time

	for offset := 0; ; offset += recentScorePageSize {
		page := osu.GetRecentScores(c.Provider.GetToken(), c.UserID, recentScorePageSize, offset)
		c.Logger.Debug("Recent scores", "Count", len(page), "Offset", offset)
		if len(page) == 0 {
			break
		}
		if offset == 0 {
			newest = page[0].CreatedAt
		}
		if !c.foldPage(page, prev) || len(page) < recentScorePageSize {
			break
		}
	}

	if newest.After(c.LastUpdate) {
		c.LastUpdate = newest
	}
	return newest.After(prev)
}

// Ingest folds a single externally-sourced score into the ranking, for callers
// that receive scores from a push stream instead of polling. Unlike Update it
// issues no osu! API request: it trusts the score's PP as given and only counts
// plays set after WindowStart, so anything predating the tracking window is
// ignored. Deduplication by score and beatmap is handled by the ranking, so
// re-delivering a score is harmless. Returns the score's rank and whether it
// entered the ranking.
func (c *Client) Ingest(score player.Score) (rank int, added bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !score.CreatedAt.After(c.WindowStart) {
		return 0, false
	}

	rank, added = c.ranking.AddScore(score)
	if !added {
		return rank, false
	}

	c.Logger.Info("Ingested score", "ID", score.ID, "BeatmapID", score.Beatmap.ID, "Title", score.BeatmapSet.Title, "PP", score.PP)
	if c.OnNewScores != nil {
		go c.OnNewScores([]NewScore{{Rank: rank, Score: score}})
	}
	return rank, true
}

// Ranking safely returns client ranking.
// Modifications in the resulting ranking will not affect client.
func (c *Client) Ranking() player.Ranking {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.ranking
}

// GetTotalPP safely returns total PP
func (c *Client) GetTotalPP() float64 {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.ranking.GetTotalPP()
}

// foldPage adds every score on the page newer than prev into the ranking. It
// stops at the first score at or older than prev. pageAllNew reports whether the
// whole page was newer than prev, meaning more new scores may sit on the next page.
func (c *Client) foldPage(page player.Scores, prev time.Time) (pageAllNew bool) {
	pageAllNew = true
	var newScores []NewScore

	for i := range page {
		score := page[i]
		if score.CreatedAt.Compare(prev) <= 0 {
			pageAllNew = false
			break
		}

		osu.GetPP(&score)
		c.Logger.Debug("Possible new score", "ID", score.ID, "BeatmapID", score.Beatmap.ID, "Title", score.BeatmapSet.Title, "PP", score.PP)
		if rank, added := c.ranking.AddScore(score); added {
			c.Logger.Info("New score", "ID", score.ID, "BeatmapID", score.Beatmap.ID, "Title", score.BeatmapSet.Title, "PP", score.PP)
			newScores = append(newScores, NewScore{Rank: rank, Score: score})
		}
	}

	if c.OnNewScores != nil && len(newScores) > 0 {
		go c.OnNewScores(newScores)
	}
	return pageAllNew
}

func (c *Client) getBeatmapScores(beatmapID int) player.Scores {
	return osu.GetBeatmapScores(c.Provider.GetToken(), c.UserID, beatmapID)
}
