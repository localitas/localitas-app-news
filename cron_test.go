package news

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCron(t *testing.T) {
	req := httptest.NewRequest("GET", "/cron.json", nil)
	w := httptest.NewRecorder()
	HandleCron(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var spec struct {
		Jobs []struct {
			ID          string `json:"id"`
			Path        string `json:"path"`
			Method      string `json:"method"`
			Schedule    string `json:"schedule"`
			Description string `json:"description"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(w.Body).Decode(&spec); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(spec.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(spec.Jobs))
	}

	job := spec.Jobs[0]
	if job.ID != "cron:news:feed-sync" {
		t.Errorf("expected id cron:news:feed-sync, got %s", job.ID)
	}
	if job.Path != "/api/feeds/sync" {
		t.Errorf("expected path /api/feeds/sync, got %s", job.Path)
	}
	if job.Method != "POST" {
		t.Errorf("expected method POST, got %s", job.Method)
	}
	if job.Schedule != "0 * * * *" {
		t.Errorf("expected schedule 0 * * * *, got %s", job.Schedule)
	}
}

func TestHandleCron_ContentType(t *testing.T) {
	req := httptest.NewRequest("GET", "/cron.json", nil)
	w := httptest.NewRecorder()
	HandleCron(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

func TestHandleCron_RetryConfig(t *testing.T) {
	req := httptest.NewRequest("GET", "/cron.json", nil)
	w := httptest.NewRecorder()
	HandleCron(w, req)

	var spec struct {
		Jobs []struct {
			Retry struct {
				MaxAttempts       int     `json:"max_attempts"`
				InitialDelay      string  `json:"initial_delay"`
				Backoff           string  `json:"backoff"`
				BackoffMultiplier float64 `json:"backoff_multiplier"`
			} `json:"retry"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(w.Body).Decode(&spec); err != nil {
		t.Fatalf("decode: %v", err)
	}

	retry := spec.Jobs[0].Retry
	if retry.MaxAttempts != 3 {
		t.Errorf("expected max_attempts 3, got %d", retry.MaxAttempts)
	}
	if retry.InitialDelay != "5s" {
		t.Errorf("expected initial_delay 5s, got %s", retry.InitialDelay)
	}
	if retry.Backoff != "exponential" {
		t.Errorf("expected backoff exponential, got %s", retry.Backoff)
	}
	if retry.BackoffMultiplier != 2.0 {
		t.Errorf("expected backoff_multiplier 2.0, got %f", retry.BackoffMultiplier)
	}
}

func TestHandleCron_MethodIsPost(t *testing.T) {
	req := httptest.NewRequest("GET", "/cron.json", nil)
	w := httptest.NewRecorder()
	HandleCron(w, req)

	var spec struct {
		Jobs []struct {
			Method string `json:"method"`
		} `json:"jobs"`
	}
	if err := json.NewDecoder(w.Body).Decode(&spec); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for i, job := range spec.Jobs {
		if job.Method != "POST" {
			t.Errorf("job[%d]: expected method POST, got %s", i, job.Method)
		}
	}
}
