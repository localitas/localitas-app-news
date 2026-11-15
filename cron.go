package news

import (
	"net/http"

	client "github.com/localitas/localitas-go"
)

var cronSpec = client.CronSpec{
	Jobs: []client.CronJob{
		{
			ID:          "cron:news:feed-sync",
			Path:        "/api/feeds/sync",
			Method:      "POST",
			Schedule:    "0 * * * *",
			Description: "Fetches new articles from all RSS/Atom feeds",
			Timeout:     "120s",
			Retry: &client.CronRetry{
				MaxAttempts:       3,
				InitialDelay:      "5s",
				MaxDelay:          "30s",
				Backoff:           "exponential",
				BackoffMultiplier: 2.0,
			},
		},
	},
}

func HandleCron(w http.ResponseWriter, r *http.Request) {
	cronSpec.ServeHTTP(w, r)
}
