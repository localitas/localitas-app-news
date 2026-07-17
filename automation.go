package news

import (
	"context"
	"fmt"

	client "github.com/localitas/localitas-go"
)

const syncAutomationName = "News: Feed Sync"

// RegisterSyncAutomation registers a periodic feed sync job with core's automation
// system. Only creates if it doesn't already exist — user modifications to the
// schedule are preserved across app restarts.
func RegisterSyncAutomation(ctx context.Context, c *client.Client, appURL string) {
	if automationExists(ctx, c, syncAutomationName) {
		logger.Info("news sync automation already registered")
		return
	}

	req := client.CreateAutomationRequest{
		Name:        syncAutomationName,
		Description: "Fetches new articles from all RSS/Atom feeds every hour",
		DAGConfig: client.DAGConfig{
			DAGID:       "news_feed_sync",
			Name:        "News: Feed Sync",
			Description: "Calls the news app sync endpoint",
			Nodes: []client.DAGNode{
				{
					NodeID:            "sync_feeds",
					NodeType:          "http-api",
					ExecutionStrategy: "raft-leader",
					Metadata: map[string]any{
						"url":                appURL + "/api/feeds/sync",
						"method":             "POST",
						"timeout_ms":         120000,
						"max_retries":        3,
						"backoff_ms":         5000,
						"backoff_multiplier": 2.0,
						"expected_status":    200,
					},
				},
			},
		},
		TriggerType: "periodic",
		TriggerConfig: client.TriggerConfig{
			Periodic: &client.PeriodicTrigger{
				Schedule:   "0 * * * *",
				Timezone:   "Local",
				MaxRetries: 2,
			},
		},
		IsEnabled: true,
	}

	if _, err := c.Automation().Create(ctx, req); err != nil {
		logger.Error("failed to register news sync automation", "error", err)
		return
	}
	logger.Info("registered news sync automation", "interval", "hourly")
}

func automationExists(ctx context.Context, c *client.Client, name string) bool {
	automations, err := c.Automation().List(ctx)
	if err != nil {
		return false
	}
	for _, a := range automations {
		if a.Name == name {
			return true
		}
	}
	return false
}

// SyncURL returns the full sync endpoint URL for this app.
func SyncURL(appURL string) string {
	return fmt.Sprintf("%s/api/feeds/sync", appURL)
}
