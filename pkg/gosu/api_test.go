package gosu

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc serves a canned response for any request, recording the last URL it
// saw so tests can assert what was requested.
type roundTripFunc struct {
	body    string
	status  int
	lastURL string
}

func (f *roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	f.lastURL = req.URL.String()
	status := f.status
	if status == 0 {
		status = 200
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}

// failRoundTrip fails the test if any request is made through it.
type failRoundTrip struct{ t *testing.T }

func (f failRoundTrip) RoundTrip(*http.Request) (*http.Response, error) {
	f.t.Fatal("unexpected API request")
	return nil, errors.New("unreachable")
}

func clientWith(rt http.RoundTripper) *Client {
	return &Client{http: &http.Client{Transport: rt}}
}

func manyIDs(n int) []int64 {
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	return ids
}

func TestBulkIDsURLRepeatsParam(t *testing.T) {
	got := bulkIDsURL("users", []int64{7, 42})
	if !strings.Contains(got, "ids%5B%5D=7") || !strings.Contains(got, "ids%5B%5D=42") {
		t.Fatalf("url %q missing repeated ids[] params", got)
	}
}

func TestGetUsersRejectsTooManyIDs(t *testing.T) {
	c := clientWith(failRoundTrip{t})
	if _, err := c.GetUsers(&GuestToken{}, manyIDs(maxBulkIDs+1)); !errors.Is(err, ErrTooManyIDs) {
		t.Fatalf("err = %v, want ErrTooManyIDs", err)
	}
}

func TestGetUsersEmptySkipsRequest(t *testing.T) {
	c := clientWith(failRoundTrip{t})
	got, err := c.GetUsers(&GuestToken{}, nil)
	if err != nil || got != nil {
		t.Fatalf("GetUsers(nil) = %v, %v; want nil, nil", got, err)
	}
}

// The bulk users endpoint reports ranking stats per ruleset; GetUsers folds the osu!
// ruleset stats onto Statistics so the shape matches the single-user endpoint.
func TestGetUsersFoldsRulesetStats(t *testing.T) {
	rt := &roundTripFunc{body: `{"users":[{"id":9,"username":"a","statistics_rulesets":{"osu":{"pp":1234.5,"global_rank":10}}}]}`}
	got, err := clientWith(rt).GetUsers(&GuestToken{}, []int64{9})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d profiles, want 1", len(got))
	}
	if got[0].Statistics.PP != 1234.5 {
		t.Fatalf("PP = %v, want 1234.5", got[0].Statistics.PP)
	}
	if got[0].GlobalRank() == nil || *got[0].GlobalRank() != 10 {
		t.Fatalf("GlobalRank = %v, want 10", got[0].GlobalRank())
	}
}

func TestGetBeatmapsRejectsTooManyIDs(t *testing.T) {
	c := clientWith(failRoundTrip{t})
	if _, err := c.GetBeatmaps(&GuestToken{}, manyIDs(maxBulkIDs+1)); !errors.Is(err, ErrTooManyIDs) {
		t.Fatalf("err = %v, want ErrTooManyIDs", err)
	}
}

func TestGetBeatmapsEmptySkipsRequest(t *testing.T) {
	c := clientWith(failRoundTrip{t})
	got, err := c.GetBeatmaps(&GuestToken{}, nil)
	if err != nil || got != nil {
		t.Fatalf("GetBeatmaps(nil) = %v, %v; want nil, nil", got, err)
	}
}

func TestGetBeatmapsDecodesSet(t *testing.T) {
	rt := &roundTripFunc{body: `{"beatmaps":[{"id":5,"max_combo":600,"beatmapset":{"id":3,"title":"t"}}]}`}
	got, err := clientWith(rt).GetBeatmaps(&GuestToken{}, []int64{5})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 5 || got[0].MaxCombo != 600 {
		t.Fatalf("beatmap = %+v, want id 5 max_combo 600", got)
	}
	if got[0].Beatmapset.ID != 3 || got[0].Beatmapset.Title != "t" {
		t.Fatalf("set = %+v, want id 3 title t", got[0].Beatmapset)
	}
}
