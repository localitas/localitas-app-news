package news

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	client "github.com/localitas/localitas-go"
	"github.com/localitas/localitas-go/identitytest"
	_ "github.com/mattn/go-sqlite3"
)

const testAPIToken = "lt_test_cluster_token"

// signer signs identity tokens as core does; the app verifies them with its
// public key and refuses legacy (api_token-derived HMAC) tokens.
var signer *identitytest.Signer

func TestMain(m *testing.M) {
	os.Setenv("LOCALITAS_API_TOKEN", testAPIToken)
	var err error
	if signer, err = identitytest.Install(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// identity signs claims as core does for a request it forwards to the app.
func identity(userID, permission string) string {
	return signer.Token(client.TokenInfo{UserID: userID, Email: userID + "@test.local", Permission: permission})
}

func newOwnershipApp(t *testing.T) (*App, *http.ServeMux) {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "news.sqlitedb")+"?_synchronous=OFF&_journal_mode=MEMORY")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	files, _ := filepath.Glob("_migrations/*.sql")
	for _, f := range files {
		sqlText, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(sqlText)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	app := New(client.New("http://127.0.0.1:1"), "/")
	app.Store = NewStore(db)
	mux := http.NewServeMux()
	app.RegisterRoutes(mux)
	return app, mux
}

func do(mux *http.ServeMux, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestCollections_BelongToTheirUser(t *testing.T) {
	app, mux := newOwnershipApp(t)
	col, err := app.Store.CreateCollection(context.Background(), "alice", "alice reading", "", "")
	if err != nil {
		t.Fatal(err)
	}
	bob := identity("bob", "write")
	if rec := do(mux, "GET", "/api/collections", bob, ""); strings.Contains(rec.Body.String(), col.ID) {
		t.Fatalf("bob lists alice's collection: %s", rec.Body)
	}
	if rec := do(mux, "DELETE", "/api/collections/"+col.ID, bob, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("bob deletes alice's collection: %d %s", rec.Code, rec.Body)
	}
	if rec := do(mux, "POST", "/api/feeds", bob, `{"url":"http://127.0.0.1:1/feed","collection_id":"`+col.ID+`"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("bob adds a feed to alice's collection: %d %s", rec.Code, rec.Body)
	}
	if rec := do(mux, "GET", "/api/collections", identity("alice", "write"), ""); !strings.Contains(rec.Body.String(), col.ID) {
		t.Fatalf("alice lost her collection: %s", rec.Body)
	}
	if rec := do(mux, "DELETE", "/api/collections/"+col.ID, identity("alice", "write"), ""); rec.Code != http.StatusOK {
		t.Fatalf("alice deletes her collection: %d %s", rec.Code, rec.Body)
	}
}

func TestNews_RequiresAVerifiedUser(t *testing.T) {
	_, mux := newOwnershipApp(t)
	forged := base64.StdEncoding.EncodeToString([]byte(`{"user_id":"alice","permission":"admin"}`))
	for _, token := range []string{"", forged} {
		for _, route := range []struct{ method, path string }{{"GET", "/api/collections"}, {"GET", "/api/feeds"}, {"POST", "/api/feeds/sync"}} {
			if rec := do(mux, route.method, route.path, token, ""); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s with %q: %d, want 401", route.method, route.path, token, rec.Code)
			}
		}
	}
	if rec := do(mux, "POST", "/api/tags", identity("reader", "read"), `{"name":"x"}`); rec.Code != http.StatusForbidden {
		t.Errorf("a read-only user creates a tag: %d, want 403", rec.Code)
	}
}
