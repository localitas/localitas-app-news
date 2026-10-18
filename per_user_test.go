package news

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	client "github.com/localitas/localitas-go"
)

const ownershipMigration = "20261018-032240-000-news_per_user_ownership.sql"

// feedServer serves a one-article RSS feed at any path; the path names the
// feed, so two paths are two feeds.
func feedServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.Trim(r.URL.Path, "/")
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprintf(w, `<?xml version="1.0"?>
<rss version="2.0"><channel><title>Feed %[1]s</title><link>http://example.test/%[1]s</link><description>the %[1]s feed</description>
<item><title>Zebra report %[1]s</title><link>http://example.test/%[1]s/1</link><guid>%[1]s-1</guid><pubDate>Mon, 05 Jan 2026 15:04:05 +0000</pubDate><description>zebra news from %[1]s</description></item>
</channel></rss>`, name)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func mustStatus(t *testing.T, rec *httptest.ResponseRecorder, want int, what string) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("%s: %d %s, want %d", what, rec.Code, rec.Body, want)
	}
}

// subscribe adds url for the user and syncs it, returning the feed and its
// one article.
func subscribe(t *testing.T, mux *http.ServeMux, token, url string) (Feed, Article) {
	t.Helper()
	rec := do(mux, "POST", "/api/feeds", token, `{"url":"`+url+`"}`)
	mustStatus(t, rec, http.StatusCreated, "subscribe to "+url)
	feed := decode[Feed](t, rec)
	mustStatus(t, do(mux, "POST", "/api/feeds/sync", testAPIToken, ""), http.StatusOK, "sync")
	articles := decode[[]Article](t, do(mux, "GET", "/api/articles?feed_id="+feed.ID, token, ""))
	if len(articles) != 1 {
		t.Fatalf("subscriber sees %d articles of %s, want 1", len(articles), url)
	}
	return feed, articles[0]
}

func feedIDs(t *testing.T, mux *http.ServeMux, token string) []string {
	t.Helper()
	var ids []string
	for _, f := range decode[[]Feed](t, do(mux, "GET", "/api/feeds", token, "")) {
		ids = append(ids, f.ID)
	}
	return ids
}

func articleFor(t *testing.T, mux *http.ServeMux, token, id string) (Article, bool) {
	t.Helper()
	for _, a := range decode[[]Article](t, do(mux, "GET", "/api/articles", token, "")) {
		if a.ID == id {
			return a, true
		}
	}
	return Article{}, false
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestFeeds_BelongToTheirSubscribers(t *testing.T) {
	_, mux := newOwnershipApp(t)
	feeds := feedServer(t)
	alice, bob := identity("alice", "write"), identity("bob", "write")

	feed, article := subscribe(t, mux, alice, feeds+"/a")

	if contains(feedIDs(t, mux, bob), feed.ID) {
		t.Fatal("bob lists alice's feed")
	}
	if _, seen := articleFor(t, mux, bob, article.ID); seen {
		t.Fatal("bob lists an article of alice's feed")
	}
	if got := decode[[]Article](t, do(mux, "GET", "/api/articles?feed_id="+feed.ID, bob, "")); len(got) != 0 {
		t.Fatalf("bob reads alice's feed by id: %d articles", len(got))
	}
	if got := decode[[]Article](t, do(mux, "GET", "/api/search?q=zebra", bob, "")); len(got) != 0 {
		t.Fatalf("bob's search finds alice's articles: %d", len(got))
	}
	mustStatus(t, do(mux, "POST", "/api/articles/"+article.ID+"/read", bob, ""), http.StatusNotFound, "bob marks alice's article read")
	mustStatus(t, do(mux, "POST", "/api/articles/"+article.ID+"/star", bob, ""), http.StatusNotFound, "bob stars alice's article")
	mustStatus(t, do(mux, "DELETE", "/api/feeds/"+feed.ID, bob, ""), http.StatusNotFound, "bob deletes alice's feed")

	got, seen := articleFor(t, mux, alice, article.ID)
	if !seen || got.IsRead || got.IsStarred {
		t.Fatalf("bob's attempts changed alice's article: seen=%v %+v", seen, got)
	}
	if !contains(feedIDs(t, mux, alice), feed.ID) {
		t.Fatal("alice lost her feed to bob's delete")
	}
	if got := decode[[]Article](t, do(mux, "GET", "/api/search?q=zebra", alice, "")); len(got) != 1 {
		t.Fatalf("alice's search finds %d of her articles, want 1", len(got))
	}
}

func TestFeeds_TwoUsersSubscribeToTheSameURL(t *testing.T) {
	_, mux := newOwnershipApp(t)
	feeds := feedServer(t)
	alice, bob := identity("alice", "write"), identity("bob", "write")

	aliceFeed, article := subscribe(t, mux, alice, feeds+"/shared")
	bobFeed, bobArticle := subscribe(t, mux, bob, feeds+"/shared")
	if bobFeed.ID != aliceFeed.ID || bobArticle.ID != article.ID {
		t.Fatalf("one URL became two feeds: %s/%s and %s/%s", aliceFeed.ID, article.ID, bobFeed.ID, bobArticle.ID)
	}

	mustStatus(t, do(mux, "POST", "/api/articles/"+article.ID+"/read", alice, ""), http.StatusOK, "alice marks read")
	mustStatus(t, do(mux, "POST", "/api/articles/"+article.ID+"/star", bob, ""), http.StatusOK, "bob stars")

	a, _ := articleFor(t, mux, alice, article.ID)
	b, _ := articleFor(t, mux, bob, article.ID)
	if !a.IsRead || a.IsStarred {
		t.Fatalf("alice's article state = read %v starred %v, want read and not starred", a.IsRead, a.IsStarred)
	}
	if b.IsRead || !b.IsStarred {
		t.Fatalf("bob's article state = read %v starred %v, want starred and not read", b.IsRead, b.IsStarred)
	}

	mustStatus(t, do(mux, "DELETE", "/api/feeds/"+aliceFeed.ID, alice, ""), http.StatusOK, "alice unsubscribes")
	if contains(feedIDs(t, mux, alice), aliceFeed.ID) {
		t.Fatal("alice still lists the feed she unsubscribed from")
	}
	if _, seen := articleFor(t, mux, bob, article.ID); !seen {
		t.Fatal("alice's unsubscribe removed bob's articles")
	}
	mustStatus(t, do(mux, "DELETE", "/api/feeds/"+aliceFeed.ID, alice, ""), http.StatusNotFound, "alice unsubscribes twice")
}

func TestTags_BelongToTheirUser(t *testing.T) {
	_, mux := newOwnershipApp(t)
	feeds := feedServer(t)
	alice, bob := identity("alice", "write"), identity("bob", "write")
	_, article := subscribe(t, mux, alice, feeds+"/tagged")
	subscribe(t, mux, bob, feeds+"/tagged")

	rec := do(mux, "POST", "/api/tags", alice, `{"name":"work"}`)
	mustStatus(t, rec, http.StatusCreated, "alice creates a tag")
	aliceTag := decode[Tag](t, rec)
	mustStatus(t, do(mux, "POST", "/api/articles/"+article.ID+"/tag", alice, `{"tag_id":"`+aliceTag.ID+`"}`), http.StatusOK, "alice tags")

	bobTags := decode[[]Tag](t, do(mux, "GET", "/api/tags", bob, ""))
	for _, tag := range bobTags {
		if tag.ID == aliceTag.ID {
			t.Fatal("bob lists alice's tag")
		}
	}
	mustStatus(t, do(mux, "POST", "/api/tags", bob, `{"name":"work"}`), http.StatusCreated, "bob names a tag like alice's")
	if b, _ := articleFor(t, mux, bob, article.ID); len(b.Tags) != 0 {
		t.Fatalf("bob sees alice's tags on the article: %v", b.Tags)
	}
	mustStatus(t, do(mux, "POST", "/api/articles/"+article.ID+"/tag", bob, `{"tag_id":"`+aliceTag.ID+`"}`), http.StatusNotFound, "bob uses alice's tag")
	mustStatus(t, do(mux, "DELETE", "/api/articles/"+article.ID+"/tags/"+aliceTag.ID, bob, ""), http.StatusNotFound, "bob untags alice's tag")
	mustStatus(t, do(mux, "GET", "/api/tags/"+aliceTag.ID+"/articles", bob, ""), http.StatusNotFound, "bob lists alice's tag")
	mustStatus(t, do(mux, "DELETE", "/api/tags/"+aliceTag.ID, bob, ""), http.StatusNotFound, "bob deletes alice's tag")

	tagged := decode[[]Article](t, do(mux, "GET", "/api/tags/"+aliceTag.ID+"/articles", alice, ""))
	if len(tagged) != 1 || tagged[0].ID != article.ID {
		t.Fatalf("alice's tag lost its article: %+v", tagged)
	}
	if a, _ := articleFor(t, mux, alice, article.ID); len(a.Tags) != 1 || a.Tags[0] != "work" {
		t.Fatalf("alice's article tags = %v, want [work]", a.Tags)
	}

	favorites := func(token string) string {
		for _, tag := range decode[[]Tag](t, do(mux, "GET", "/api/tags", token, "")) {
			if tag.Name == "Favorites" {
				return tag.ID
			}
		}
		return ""
	}
	aliceFav, bobFav := favorites(alice), favorites(bob)
	if aliceFav == "" || bobFav == "" || aliceFav == bobFav {
		t.Fatalf("each user needs their own Favorites tag: alice %q bob %q", aliceFav, bobFav)
	}
	if again := favorites(alice); again != aliceFav {
		t.Fatalf("listing tags again made a second Favorites: %q then %q", aliceFav, again)
	}
}

// openLegacyDB is a news database from before per-user ownership: every
// migration but the ownership one, holding a household's feed, read and
// starred article, and tag.
func openLegacyDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "news.sqlitedb")+"?_synchronous=OFF&_journal_mode=MEMORY")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	files, _ := filepath.Glob("_migrations/*.sql")
	for _, f := range files {
		if filepath.Base(f) == ownershipMigration {
			continue
		}
		applyMigration(t, db, f)
	}
	now := time.Now().UTC().Unix()
	for _, stmt := range []string{
		fmt.Sprintf(`INSERT INTO feeds (id, url, title, created_at, updated_at) VALUES ('legacy-feed', 'http://legacy.test/rss', 'Legacy', %d, %d)`, now, now),
		fmt.Sprintf(`INSERT INTO articles (id, feed_id, guid, title, link, published_at, fetched_at, is_read, is_starred) VALUES ('legacy-article', 'legacy-feed', 'g1', 'Old news', 'http://legacy.test/1', %d, %d, 1, 1)`, now, now),
		fmt.Sprintf(`INSERT INTO tags (id, name, color, created_at) VALUES ('legacy-tag', 'keep', '#000000', %d)`, now),
		fmt.Sprintf(`INSERT INTO article_tags (article_id, tag_id, created_at) VALUES ('legacy-article', 'legacy-tag', %d)`, now),
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	applyMigration(t, db, filepath.Join("_migrations", ownershipMigration))
	return db, "legacy-feed"
}

func applyMigration(t *testing.T, db *sql.DB, path string) {
	t.Helper()
	sqlText, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(sqlText)); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func appOn(db *sql.DB) *http.ServeMux {
	app := New(client.New("http://127.0.0.1:1"), "/")
	app.Store = NewStore(db)
	mux := http.NewServeMux()
	app.RegisterRoutes(mux)
	return mux
}

// What the household read before ownership goes to the first admin who
// signs in, and to nobody else, even across restarts.
func TestLegacyData_GoesToTheFirstAdminOnce(t *testing.T) {
	db, legacyFeed := openLegacyDB(t)
	mux := appOn(db)
	bob, root, root2 := identity("bob", "write"), identity("root", "admin"), identity("root2", "admin")

	if contains(feedIDs(t, mux, bob), legacyFeed) {
		t.Fatal("a non-admin was given the household's legacy feeds")
	}
	if !contains(feedIDs(t, mux, root), legacyFeed) {
		t.Fatal("the first admin did not get the legacy feeds")
	}
	a, seen := articleFor(t, mux, root, "legacy-article")
	if !seen || !a.IsRead || !a.IsStarred || len(a.Tags) != 1 || a.Tags[0] != "keep" {
		t.Fatalf("the admin's legacy article lost its state: seen=%v %+v", seen, a)
	}
	var names []string
	for _, tag := range decode[[]Tag](t, do(mux, "GET", "/api/tags", root, "")) {
		names = append(names, tag.Name)
	}
	if strings.Join(names, ",") != "Favorites,keep" {
		t.Fatalf("the admin's tags = %v, want the legacy Favorites and keep, once each", names)
	}

	restarted := appOn(db)
	if contains(feedIDs(t, restarted, root2), legacyFeed) {
		t.Fatal("a second admin was given the legacy feeds after a restart")
	}
	if contains(feedIDs(t, restarted, bob), legacyFeed) {
		t.Fatal("bob was given the legacy feeds after a restart")
	}
	if !contains(feedIDs(t, restarted, root), legacyFeed) {
		t.Fatal("the first admin lost the legacy feeds after a restart")
	}
	var unowned int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM feed_subscriptions WHERE user_id = ''`).Scan(&unowned); err != nil {
		t.Fatal(err)
	}
	if unowned != 0 {
		t.Fatalf("%d legacy subscriptions are still unowned", unowned)
	}
}
