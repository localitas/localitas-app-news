package news

import (
	"testing"
)

func TestParseRSSFeed(t *testing.T) {
	xml := `<?xml version="1.0"?>
<rss version="2.0">
<channel>
<title>Test Feed</title>
<description>A test feed</description>
<link>https://example.com</link>
<item>
<title>Article One</title>
<link>https://example.com/1</link>
<description>First article</description>
<pubDate>Mon, 01 Jan 2026 12:00:00 +0000</pubDate>
<guid>1</guid>
</item>
<item>
<title>Article Two</title>
<link>https://example.com/2</link>
<description>Second article</description>
<pubDate>Tue, 02 Jan 2026 12:00:00 +0000</pubDate>
<guid>2</guid>
</item>
</channel>
</rss>`

	feed, articles, err := ParseFeedFromBytes([]byte(xml))
	if err != nil {
		t.Fatalf("ParseFeedFromBytes: %v", err)
	}
	if feed.Title != "Test Feed" {
		t.Errorf("expected title 'Test Feed', got %q", feed.Title)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}
	if articles[0].Title != "Article One" {
		t.Errorf("expected 'Article One', got %q", articles[0].Title)
	}
}

func TestParseAtomFeed(t *testing.T) {
	xml := `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
<title>Atom Feed</title>
<link href="https://example.com" rel="alternate"/>
<entry>
<title>Entry One</title>
<link href="https://example.com/e1" rel="alternate"/>
<id>e1</id>
<summary>First entry</summary>
<updated>2026-01-01T12:00:00Z</updated>
</entry>
</feed>`

	feed, articles, err := ParseFeedFromBytes([]byte(xml))
	if err != nil {
		t.Fatalf("ParseFeedFromBytes: %v", err)
	}
	if feed.Title != "Atom Feed" {
		t.Errorf("expected 'Atom Feed', got %q", feed.Title)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
}

func TestCleanHTML(t *testing.T) {
	input := "<p>Hello <b>world</b></p><script>evil()</script>"
	got := cleanHTML(input)
	if got != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", got)
	}
}

func TestGenerateGUID(t *testing.T) {
	g1 := generateGUID("test")
	g2 := generateGUID("test")
	if g1 != g2 {
		t.Error("same input should produce same GUID")
	}
	g3 := generateGUID("different")
	if g1 == g3 {
		t.Error("different input should produce different GUID")
	}
}

func TestHandleSwagger(t *testing.T) {
	// Already covered by apidoc_test pattern — just verify it compiles
	if len(NewsAPIDoc.Endpoints) == 0 {
		t.Error("expected endpoints in NewsAPIDoc")
	}
}
