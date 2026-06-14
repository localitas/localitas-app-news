package news

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

type APIEndpoint struct {
	Method      string     `json:"method"`
	Path        string     `json:"path"`
	Summary     string     `json:"summary"`
	QueryParams []APIParam `json:"query_params,omitempty"`
	RequestBody *APIBody   `json:"request_body,omitempty"`
	Response    *APIBody   `json:"response,omitempty"`
}

type APIParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type APIBody struct {
	ContentType string `json:"content_type"`
	Example     string `json:"example"`
}

type APIFieldDoc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

type APIDoc struct {
	AppName     string        `json:"app_name"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	Keywords    []string      `json:"keywords,omitempty"`
	Fields      []APIFieldDoc `json:"fields,omitempty"`
	Endpoints   []APIEndpoint `json:"endpoints"`
}

var NewsAPIDoc = APIDoc{
	AppName:     "News",
	Version:     "0.1.0",
	Description: "RSS/Atom feed reader with collections, sync, and full-text search",
	Keywords:    []string{"news", "feed", "RSS", "atom", "article", "headline", "blog", "subscribe", "syndication", "reader"},
	Fields: []APIFieldDoc{
		{Name: "Collections", Description: "Folders for organizing feeds", Example: "Tech News\nDev Blogs\nDesign"},
		{Name: "Feeds", Description: "RSS/Atom feed URLs to subscribe to", Example: "https://hnrss.org/frontpage\nhttps://blog.golang.org/feed.atom"},
		{Name: "Article States", Description: "Articles can be marked as read or starred", Example: "is_read: true/false\nis_starred: true/false"},
	},
	Endpoints: []APIEndpoint{
		{Method: "GET", Path: "/api/collections", Summary: "List all collections", Response: &APIBody{ContentType: "application/json", Example: `[{"id":"abc...","name":"Tech News","feed_count":3}]`}},
		{Method: "POST", Path: "/api/collections", Summary: "Create a collection", RequestBody: &APIBody{ContentType: "application/json", Example: `{"name":"Tech News","description":"Technology feeds","icon":"rss"}`}, Response: &APIBody{ContentType: "application/json", Example: `{"id":"abc...","name":"Tech News"}`}},
		{Method: "DELETE", Path: "/api/collections/{id}", Summary: "Delete a collection", Response: &APIBody{ContentType: "application/json", Example: `{"success":true}`}},
		{Method: "GET", Path: "/api/feeds", Summary: "List all feeds", Response: &APIBody{ContentType: "application/json", Example: `[{"id":"abc...","url":"https://...","title":"Hacker News","is_active":true}]`}},
		{Method: "POST", Path: "/api/feeds", Summary: "Add a feed (auto-fetches title)", RequestBody: &APIBody{ContentType: "application/json", Example: `{"url":"https://hnrss.org/frontpage","collection_id":"abc..."}`}, Response: &APIBody{ContentType: "application/json", Example: `{"id":"abc...","url":"https://...","title":"Hacker News"}`}},
		{Method: "DELETE", Path: "/api/feeds/{id}", Summary: "Remove a feed", Response: &APIBody{ContentType: "application/json", Example: `{"success":true}`}},
		{Method: "POST", Path: "/api/feeds/sync", Summary: "Sync all feeds (fetch new articles)", Response: &APIBody{ContentType: "application/json", Example: `{"total_feeds":5,"success_count":5,"new_articles":23,"duration_ms":1200}`}},
		{Method: "GET", Path: "/api/articles", Summary: "List articles", QueryParams: []APIParam{{Name: "feed_id", Type: "string", Description: "Filter by feed"}, {Name: "limit", Type: "integer", Description: "Max results (default 50)"}, {Name: "offset", Type: "integer", Description: "Offset"}}, Response: &APIBody{ContentType: "application/json", Example: `[{"id":"abc...","title":"Article Title","feed_title":"HN","link":"https://..."}]`}},
		{Method: "POST", Path: "/api/articles/{id}/read", Summary: "Mark article as read", Response: &APIBody{ContentType: "application/json", Example: `{"success":true}`}},
		{Method: "POST", Path: "/api/articles/{id}/star", Summary: "Toggle article star", Response: &APIBody{ContentType: "application/json", Example: `{"success":true}`}},
		{Method: "GET", Path: "/api/search", Summary: "Search articles (FTS5)", QueryParams: []APIParam{{Name: "q", Type: "string", Required: true, Description: "Search query"}}, Response: &APIBody{ContentType: "application/json", Example: `[{"id":"abc...","title":"Matching Article","description":"..."}]`}},
		{Method: "GET", Path: "/api/defaults", Summary: "List default feed suggestions", Response: &APIBody{ContentType: "application/json", Example: `[{"url":"https://hnrss.org/frontpage","title":"Hacker News","category":"Tech"}]`}},
		{Method: "GET", Path: "/api/tags", Summary: "List all tags", Response: &APIBody{ContentType: "application/json", Example: `[{"id":"abc...","name":"Favorites","color":"#f59e0b"}]`}},
		{Method: "POST", Path: "/api/tags", Summary: "Create a tag", RequestBody: &APIBody{ContentType: "application/json", Example: `{"name":"Read Later","color":"#60a5fa"}`}, Response: &APIBody{ContentType: "application/json", Example: `{"id":"abc...","name":"Read Later","color":"#60a5fa"}`}},
		{Method: "DELETE", Path: "/api/tags/{id}", Summary: "Delete a tag", Response: &APIBody{ContentType: "application/json", Example: `{"success":true}`}},
		{Method: "POST", Path: "/api/articles/{id}/tag", Summary: "Tag an article", RequestBody: &APIBody{ContentType: "application/json", Example: `{"tag_id":"abc..."}`}, Response: &APIBody{ContentType: "application/json", Example: `{"success":true}`}},
		{Method: "DELETE", Path: "/api/articles/{id}/tags/{tag_id}", Summary: "Remove tag from article", Response: &APIBody{ContentType: "application/json", Example: `{"success":true}`}},
		{Method: "GET", Path: "/api/tags/{id}/articles", Summary: "List articles by tag", QueryParams: []APIParam{{Name: "limit", Type: "integer", Description: "Max results"}, {Name: "offset", Type: "integer", Description: "Offset"}}, Response: &APIBody{ContentType: "application/json", Example: `[{"id":"abc...","title":"Tagged Article"}]`}},
	},
}

func HandleSwagger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(NewsAPIDoc)
}

func RenderDocsHTML(doc APIDoc) template.HTML {
	var sb strings.Builder
	if len(doc.Fields) > 0 {
		sb.WriteString(`<h3 style="font-size: 0.875rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--color-text-secondary); margin-bottom: 1rem;">Reference</h3><div class="accordion-list">`)
		for _, f := range doc.Fields {
			sb.WriteString(fmt.Sprintf(`<details class="glass-panel" style="border-radius: 0.5rem; margin-bottom: 0.5rem;"><summary style="padding: 0.75rem 1rem; cursor: pointer; font-weight: 500; color: var(--color-text-primary);">%s</summary><div style="padding: 0 1rem 0.75rem; font-size: 0.875rem; color: var(--color-text-secondary);"><p style="margin-bottom: 0.5rem;">%s</p><pre style="background: var(--color-bg-base); padding: 0.75rem; border-radius: 0.375rem; overflow-x: auto; font-size: 0.8125rem;">%s</pre></div></details>`, template.HTMLEscapeString(f.Name), template.HTMLEscapeString(f.Description), template.HTMLEscapeString(f.Example)))
		}
		sb.WriteString(`</div><hr style="border-color: var(--color-glass-border); margin: 1.5rem 0;">`)
	}
	sb.WriteString(`<h3 style="font-size: 0.875rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--color-text-secondary); margin-bottom: 1rem;">API Endpoints</h3><div class="accordion-list">`)
	for _, ep := range doc.Endpoints {
		title := fmt.Sprintf("%s %s — %s", ep.Method, ep.Path, ep.Summary)
		sb.WriteString(fmt.Sprintf(`<details class="glass-panel" style="border-radius: 0.5rem; margin-bottom: 0.5rem;"><summary style="padding: 0.75rem 1rem; cursor: pointer; font-weight: 500; color: var(--color-text-primary);">%s</summary><div style="padding: 0 1rem 0.75rem; font-size: 0.875rem; color: var(--color-text-secondary);">`, template.HTMLEscapeString(title)))
		var ex strings.Builder
		if ep.RequestBody != nil {
			ex.WriteString("# Request\n")
			ex.WriteString(prettyJSON(ep.RequestBody.Example))
			ex.WriteString("\n\n")
		}
		if ep.Response != nil {
			ex.WriteString("# Response\n")
			ex.WriteString(prettyJSON(ep.Response.Example))
		}
		if ex.Len() > 0 {
			sb.WriteString(fmt.Sprintf(`<pre style="background: var(--color-bg-base); padding: 0.75rem; border-radius: 0.375rem; overflow-x: auto; font-size: 0.8125rem;">%s</pre>`, template.HTMLEscapeString(ex.String())))
		}
		sb.WriteString(`</div></details>`)
	}
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

func prettyJSON(s string) string {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
