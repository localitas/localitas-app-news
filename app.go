package news

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/localitas/localitas-go"
)

var logger = slog.Default().With("component", "news")

type App struct {
	Store    *Store
	BasePath string
	client   *client.Client
	// legacyClaimed is set once an admin has been given the data from
	// before per-user ownership (see Store.ClaimLegacyData).
	legacyClaimed atomic.Bool
}

func New(c *client.Client, basePath string) *App {
	if basePath == "" {
		basePath = "/"
	}
	return &App{BasePath: basePath, client: c}
}

func (a *App) InitStore(coreURL, dbID, token string) error {
	store, err := OpenStore(coreURL, dbID, token)
	if err != nil {
		return err
	}
	a.Store = store
	return nil
}

func (a *App) Install(ctx context.Context) (string, error) {
	for attempt := 1; ; attempt++ {
		db, err := a.client.CreateSystemDatabase(ctx, DatabaseName)
		if err != nil {
			logger.Warn("install attempt failed", "attempt", attempt, "error", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if err := applyEmbeddedMigrations(ctx, a.client, db.ID); err != nil {
			logger.Warn("install migrations attempt failed", "attempt", attempt, "error", err)
			time.Sleep(2 * time.Second)
			continue
		}
		return db.ID, nil
	}
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(TemplatesFS, "templates/index.html")
	if err != nil {
		logger.Error("index template error", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "index.html", map[string]interface{}{
		"BasePath": a.BasePath,
		"DocsHTML": RenderDocsHTML(NewsAPIDoc),
	})
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	h := &handler{app: a}

	mux.HandleFunc("GET /{$}", a.handleIndex)
	mux.HandleFunc("GET /swagger.json", HandleSwagger)
	mux.HandleFunc("GET /help.md", handleHelpMarkdown)
	// Subscriptions, read and starred state, tags and collections belong to
	// the user core vouches for; feeds and articles are a shared fetch cache
	// reached only through a subscription. Every route needs a verified
	// user, and changes need write scope.
	user := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, client.AuthMiddleware(a.claimLegacyData(handler)))
	}
	writer := func(pattern string, handler http.HandlerFunc) {
		user(pattern, client.RequireScopeFunc(client.ScopeWrite, handler))
	}
	user("GET /api/collections", h.handleListCollections)
	writer("POST /api/collections", h.handleCreateCollection)
	writer("DELETE /api/collections/{id}", h.handleDeleteCollection)
	user("GET /api/feeds", h.handleListFeeds)
	writer("POST /api/feeds", h.handleAddFeed)
	writer("DELETE /api/feeds/{id}", h.handleDeleteFeed)
	// Sync serves core's hourly cron and a user's refresh button.
	syncAsUser := client.AuthMiddleware(a.claimLegacyData(client.RequireScopeFunc(client.ScopeWrite, h.handleRefreshFeed)))
	mux.HandleFunc("POST /api/feeds/sync", func(w http.ResponseWriter, r *http.Request) {
		if client.IsSystemRequest(r) {
			h.handleRefreshFeed(w, r)
			return
		}
		syncAsUser.ServeHTTP(w, r)
	})
	user("GET /api/articles", h.handleListArticles)
	writer("POST /api/articles/{id}/read", h.handleMarkRead)
	writer("POST /api/articles/{id}/star", h.handleToggleStar)
	user("GET /api/search", h.handleSearch)
	user("GET /api/defaults", h.handleListDefaults)
	user("GET /api/tags", h.handleListTags)
	writer("POST /api/tags", h.handleCreateTag)
	writer("DELETE /api/tags/{id}", h.handleDeleteTag)
	writer("POST /api/articles/{id}/tag", h.handleTagArticle)
	writer("DELETE /api/articles/{id}/tags/{tag_id}", h.handleUntagArticle)
	user("GET /api/tags/{id}/articles", h.handleListArticlesByTag)
}

// claimLegacyData gives the data from before per-user ownership to the first
// admin core vouches for. ListUsers does not say who is an admin, so the
// first verified admin request is where the app learns it. A failed claim
// is retried on that admin's next request.
func (a *App) claimLegacyData(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.legacyClaimed.Load() && client.GetScope(r.Context()) == client.ScopeAdmin {
			if err := a.Store.ClaimLegacyData(r.Context(), client.GetUserID(r.Context())); err != nil {
				logger.Error("legacy data not claimed", "user_id", client.GetUserID(r.Context()), "error", err)
			} else {
				a.legacyClaimed.Store(true)
			}
		}
		next(w, r)
	}
}
