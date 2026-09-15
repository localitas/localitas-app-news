package news

import (
	"net/http"
	"testing"
	"time"
)

// FetchAndParseFeed used to allocate a fresh http.Client per feed, per sync
// cycle (in a 5-way fan-out). It now shares one pooled client. Assert the shared
// client exists, is bounded, and its transport allows connection reuse across
// the concurrent feed workers.
func TestSharedFeedHTTPClient(t *testing.T) {
	if httpClient == nil {
		t.Fatal("feed fetches must share a pooled http client")
	}
	if httpClient.Timeout <= 0 || httpClient.Timeout > 2*time.Minute {
		t.Fatalf("shared client timeout = %v, want a sane positive bound", httpClient.Timeout)
	}
	tr, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected a configured *http.Transport for connection reuse")
	}
	if tr.MaxIdleConnsPerHost < 2 {
		t.Errorf("MaxIdleConnsPerHost = %d, want >= 2 so concurrent feed workers reuse connections", tr.MaxIdleConnsPerHost)
	}
}
