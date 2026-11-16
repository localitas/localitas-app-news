package news

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// A feed whose server fails for a moment is asked again, through the SDK's
// upstream client, which is pooled and drops an idle connection a NAT lost.
func TestFetchAndParseFeed_AMomentaryFailureIsAskedAgain(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Zebra</title><item><title>a</title><guid>1</guid></item></channel></rss>`))
	}))
	t.Cleanup(srv.Close)

	if feeds.Client != nil {
		t.Fatal("feeds bring their own client, without the SDK's stale-connection check")
	}
	parsed, articles, err := FetchAndParseFeed(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("after one failure: %v", err)
	}
	if parsed.Title != "Zebra" || len(articles) != 1 || calls.Load() != 2 {
		t.Errorf("feed %q, %d articles after %d calls", parsed.Title, len(articles), calls.Load())
	}
}
