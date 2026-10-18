package news

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/localitas/localitas-go"
	"github.com/localitas/localitas-go/httputil"
)

type handler struct {
	app *App
}

func (h *handler) handleListCollections(w http.ResponseWriter, r *http.Request) {
	userID := client.GetUserID(r.Context())
	cols, err := h.app.Store.ListCollections(r.Context(), userID)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, r, http.StatusOK, cols)
}

func (h *handler) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, "invalid body")
		return
	}
	userID := client.GetUserID(r.Context())
	col, err := h.app.Store.CreateCollection(r.Context(), userID, req.Name, req.Description, req.Icon)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, r, http.StatusCreated, col)
}

func (h *handler) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.app.Store.DeleteCollection(r.Context(), client.GetUserID(r.Context()), id); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *handler) handleListFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.app.Store.ListFeeds(r.Context(), client.GetUserID(r.Context()))
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, r, http.StatusOK, feeds)
}

func (h *handler) handleAddFeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL          string `json:"url"`
		CollectionID string `json:"collection_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, "invalid body")
		return
	}
	if req.URL == "" {
		writeErr(w, r, http.StatusBadRequest, "url is required")
		return
	}
	userID := client.GetUserID(r.Context())
	if req.CollectionID != "" {
		if err := h.app.Store.OwnsCollection(r.Context(), userID, req.CollectionID); err != nil {
			writeStoreErr(w, r, err)
			return
		}
	}
	parsed, _, err := FetchAndParseFeed(req.URL)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "failed to fetch feed: %v", err)
		return
	}
	feed, err := h.app.Store.AddFeed(r.Context(), userID, req.URL, parsed.Title, req.CollectionID)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, feed)
}

func (h *handler) handleDeleteFeed(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Store.DeleteFeed(r.Context(), client.GetUserID(r.Context()), r.PathValue("id")); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *handler) handleRefreshFeed(w http.ResponseWriter, r *http.Request) {
	work := func(ctx context.Context) (map[string]interface{}, error) {
		result, err := h.app.Store.SyncAllFeeds(ctx)
		if err != nil {
			return nil, err
		}
		data, _ := json.Marshal(result)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		return m, nil
	}

	if h.app.client.RunAsync(w, r, work) {
		return
	}

	result, err := h.app.Store.SyncAllFeeds(r.Context())
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, r, http.StatusOK, result)
}

func (h *handler) handleListArticles(w http.ResponseWriter, r *http.Request) {
	feedID := r.URL.Query().Get("feed_id")
	limit := intParam(r, "limit", 50)
	offset := intParam(r, "offset", 0)
	articles, err := h.app.Store.ListArticles(r.Context(), client.GetUserID(r.Context()), feedID, limit, offset)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, r, http.StatusOK, articles)
}

func (h *handler) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Store.MarkArticleRead(r.Context(), client.GetUserID(r.Context()), r.PathValue("id")); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *handler) handleToggleStar(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Store.ToggleArticleStar(r.Context(), client.GetUserID(r.Context()), r.PathValue("id")); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeErr(w, r, http.StatusBadRequest, "q is required")
		return
	}
	articles, err := h.app.Store.SearchArticles(r.Context(), client.GetUserID(r.Context()), q, 20)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, r, http.StatusOK, articles)
}

func (h *handler) handleListDefaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, DefaultFeeds)
}

func (h *handler) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.app.Store.ListTags(r.Context(), client.GetUserID(r.Context()))
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, r, http.StatusOK, tags)
}

func (h *handler) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Name == "" {
		writeErr(w, r, http.StatusBadRequest, "name is required")
		return
	}
	tag, err := h.app.Store.CreateTag(r.Context(), client.GetUserID(r.Context()), req.Name, req.Color)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, r, http.StatusCreated, tag)
}

func (h *handler) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Store.DeleteTag(r.Context(), client.GetUserID(r.Context()), r.PathValue("id")); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *handler) handleTagArticle(w http.ResponseWriter, r *http.Request) {
	articleID := r.PathValue("id")
	var req struct {
		TagID string `json:"tag_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TagID == "" {
		writeErr(w, r, http.StatusBadRequest, "tag_id is required")
		return
	}
	if err := h.app.Store.TagArticle(r.Context(), client.GetUserID(r.Context()), articleID, req.TagID); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *handler) handleUntagArticle(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Store.UntagArticle(r.Context(), client.GetUserID(r.Context()), r.PathValue("id"), r.PathValue("tag_id")); err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *handler) handleListArticlesByTag(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 50)
	offset := intParam(r, "offset", 0)
	articles, err := h.app.Store.ListArticlesByTag(r.Context(), client.GetUserID(r.Context()), r.PathValue("id"), limit, offset)
	if err != nil {
		writeStoreErr(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, articles)
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, v interface{}) {
	httputil.WriteResponse(w, r, status, v)
}

func writeErr(w http.ResponseWriter, r *http.Request, status int, format string, args ...interface{}) {
	httputil.WriteError(w, r, status, format, args...)
}

func intParam(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// writeStoreErr answers 404 for anything that is not the caller's, 500 for
// anything else.
func writeStoreErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrNotFound) {
		writeErr(w, r, http.StatusNotFound, "not found")
		return
	}
	writeErr(w, r, http.StatusInternalServerError, "%v", err)
}
