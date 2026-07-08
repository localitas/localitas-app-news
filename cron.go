package news

import (
	"encoding/json"
	"net/http"
)

func HandleCron(w http.ResponseWriter, r *http.Request) {
	spec := map[string]interface{}{
		"jobs": []map[string]interface{}{
			{
				"id":          "cron:news:feed-sync",
				"path":        "/api/feeds/sync",
				"method":      "POST",
				"schedule":    "0 * * * *",
				"description": "Fetches new articles from all RSS/Atom feeds",
				"timeout":     "120s",
				"retry": map[string]interface{}{
					"max_attempts":       3,
					"initial_delay":      "5s",
					"max_delay":          "30s",
					"backoff":            "exponential",
					"backoff_multiplier": 2.0,
				},
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}
