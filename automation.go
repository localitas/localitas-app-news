package news

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const syncAutomationName = "News: Feed Sync"

// RegisterSyncAutomation registers a periodic feed sync job with core's automation
// system via its HTTP API. Only creates if it doesn't already exist — user
// modifications to the schedule are preserved across app restarts.
func RegisterSyncAutomation(coreURL, token, appURL string) {
	if automationExists(coreURL, token, syncAutomationName) {
		logger.Info("news sync automation already registered")
		return
	}

	body := map[string]interface{}{
		"name":        syncAutomationName,
		"description": "Fetches new articles from all RSS/Atom feeds every hour",
		"dag_config": map[string]interface{}{
			"dag_id":      "news_feed_sync",
			"name":        "News: Feed Sync",
			"description": "Calls the news app sync endpoint",
			"nodes": []map[string]interface{}{
				{
					"node_id":            "sync_feeds",
					"node_type":          "http-api",
					"execution_strategy": "raft-leader",
					"metadata": map[string]interface{}{
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
		"trigger_type": "periodic",
		"trigger_config": map[string]interface{}{
			"periodic": map[string]interface{}{
				"schedule":    "0 * * * *",
				"timezone":    "Local",
				"max_retries": 2,
			},
		},
		"is_enabled": true,
	}

	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", coreURL+"/apps/automation/api/automations", bytes.NewReader(b))
	if err != nil {
		logger.Error("failed to create automation registration request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("failed to register news sync automation", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		logger.Info("registered news sync automation", "interval", "hourly")
	} else {
		logger.Warn("automation registration returned unexpected status", "status", resp.StatusCode)
	}
}

// automationExists checks if an automation with the given name already exists.
func automationExists(coreURL, token, name string) bool {
	req, err := http.NewRequest("GET", coreURL+"/apps/automation/api/automations", nil)
	if err != nil {
		return false
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result struct {
		Automations []struct {
			Name string `json:"name"`
		} `json:"automations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	for _, a := range result.Automations {
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
