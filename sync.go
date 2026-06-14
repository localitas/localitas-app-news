package news

import (
	"context"
	"log"
	"sync"
	"time"
)

type SyncResult struct {
	TotalFeeds   int              `json:"total_feeds"`
	SuccessCount int              `json:"success_count"`
	FailureCount int              `json:"failure_count"`
	NewArticles  int              `json:"new_articles"`
	FeedResults  []FeedSyncResult `json:"feed_results"`
	DurationMs   int64            `json:"duration_ms"`
}

type FeedSyncResult struct {
	FeedID      string `json:"feed_id"`
	FeedTitle   string `json:"feed_title"`
	Success     bool   `json:"success"`
	NewArticles int    `json:"new_articles"`
	Error       string `json:"error,omitempty"`
}

func (s *Store) SyncAllFeeds(ctx context.Context) (*SyncResult, error) {
	feeds, err := s.ListActiveFeeds(ctx)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{TotalFeeds: len(feeds), FeedResults: make([]FeedSyncResult, 0, len(feeds))}
	start := time.Now()

	if len(feeds) == 0 {
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	resultsChan := make(chan FeedSyncResult, len(feeds))
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, feed := range feeds {
		wg.Add(1)
		go func(f *Feed) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			resultsChan <- s.syncSingleFeed(ctx, f)
		}(feed)
	}

	go func() { wg.Wait(); close(resultsChan) }()

	for fr := range resultsChan {
		result.FeedResults = append(result.FeedResults, fr)
		if fr.Success {
			result.SuccessCount++
			result.NewArticles += fr.NewArticles
		} else {
			result.FailureCount++
		}
	}

	result.DurationMs = time.Since(start).Milliseconds()
	log.Printf("Feed sync: %d/%d feeds, %d new articles, %dms", result.SuccessCount, result.TotalFeeds, result.NewArticles, result.DurationMs)
	return result, nil
}

func (s *Store) syncSingleFeed(ctx context.Context, feed *Feed) FeedSyncResult {
	fr := FeedSyncResult{FeedID: feed.ID, FeedTitle: feed.Title}

	parsed, articles, err := FetchAndParseFeed(feed.URL)
	if err != nil {
		fr.Error = err.Error()
		s.UpdateFeedError(ctx, feed.ID, err.Error())
		return fr
	}

	s.UpdateFeedAfterSync(ctx, feed.ID, parsed.Title, parsed.Description, parsed.SiteURL, parsed.ImageURL)

	for _, a := range articles {
		inserted, _ := s.InsertArticleIfNew(ctx, feed.ID, a)
		if inserted {
			fr.NewArticles++
		}
	}

	fr.Success = true
	return fr
}
