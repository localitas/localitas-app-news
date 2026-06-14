package news

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/localitas/localitas-go"
)

type App struct {
	Store    *Store
	BasePath string
	client   *client.Client
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
			log.Printf("install: attempt %d failed (retrying): %v", attempt, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if err := applyEmbeddedMigrations(ctx, a.client, db.ID); err != nil {
			log.Printf("install: migrations attempt %d failed (retrying): %v", attempt, err)
			time.Sleep(2 * time.Second)
			continue
		}
		return db.ID, nil
	}
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(TemplatesFS, "templates/index.html")
	if err != nil {
		log.Printf("news index template error: %v", err)
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
	mux.HandleFunc("GET /api/collections", h.handleListCollections)
	mux.HandleFunc("POST /api/collections", h.handleCreateCollection)
	mux.HandleFunc("DELETE /api/collections/{id}", h.handleDeleteCollection)
	mux.HandleFunc("GET /api/feeds", h.handleListFeeds)
	mux.HandleFunc("POST /api/feeds", h.handleAddFeed)
	mux.HandleFunc("DELETE /api/feeds/{id}", h.handleDeleteFeed)
	mux.HandleFunc("POST /api/feeds/sync", h.handleRefreshFeed)
	mux.HandleFunc("GET /api/articles", h.handleListArticles)
	mux.HandleFunc("POST /api/articles/{id}/read", h.handleMarkRead)
	mux.HandleFunc("POST /api/articles/{id}/star", h.handleToggleStar)
	mux.HandleFunc("GET /api/search", h.handleSearch)
	mux.HandleFunc("GET /api/defaults", h.handleListDefaults)
	mux.HandleFunc("GET /api/tags", h.handleListTags)
	mux.HandleFunc("POST /api/tags", h.handleCreateTag)
	mux.HandleFunc("DELETE /api/tags/{id}", h.handleDeleteTag)
	mux.HandleFunc("POST /api/articles/{id}/tag", h.handleTagArticle)
	mux.HandleFunc("DELETE /api/articles/{id}/tags/{tag_id}", h.handleUntagArticle)
	mux.HandleFunc("GET /api/tags/{id}/articles", h.handleListArticlesByTag)
}
