package news

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RSS struct {
	XMLName xml.Name   `xml:"rss"`
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title       string    `xml:"title"`
	Description string    `xml:"description"`
	Link        string    `xml:"link"`
	Image       *RSSImage `xml:"image"`
	Items       []RSSItem `xml:"item"`
}

type RSSImage struct {
	URL string `xml:"url"`
}

type RSSItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	PubDate     string   `xml:"pubDate"`
	Author      string   `xml:"author"`
	Creator     string   `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Categories  []string `xml:"category"`
	GUID        string   `xml:"guid"`
	Content     string   `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	Enclosure   struct {
		URL  string `xml:"url,attr"`
		Type string `xml:"type,attr"`
	} `xml:"enclosure"`
}

type Atom struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	Link    []AtomLink  `xml:"link"`
	Entries []AtomEntry `xml:"entry"`
	Icon    string      `xml:"icon"`
}

type AtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type AtomEntry struct {
	Title   string     `xml:"title"`
	Link    []AtomLink `xml:"link"`
	ID      string     `xml:"id"`
	Summary string     `xml:"summary"`
	Content string     `xml:"content"`
	Updated string     `xml:"updated"`
	Author  struct {
		Name string `xml:"name"`
	} `xml:"author"`
}

type ParsedFeed struct {
	Title       string
	Description string
	SiteURL     string
	ImageURL    string
}

type ParsedArticle struct {
	GUID        string
	Title       string
	Description string
	Content     string
	Link        string
	Author      string
	ImageURL    string
	PublishedAt time.Time
}

func FetchAndParseFeed(feedURL string) (*ParsedFeed, []*ParsedArticle, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Localitas News/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("feed returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read feed: %w", err)
	}
	return ParseFeedFromBytes(body)
}

func ParseFeedFromBytes(body []byte) (*ParsedFeed, []*ParsedArticle, error) {
	var rss RSS
	if err := xml.Unmarshal(body, &rss); err == nil && rss.Channel.Title != "" {
		return parseRSS(&rss)
	}
	var atom Atom
	if err := xml.Unmarshal(body, &atom); err == nil && atom.Title != "" {
		return parseAtom(&atom)
	}
	return nil, nil, fmt.Errorf("unsupported feed format")
}

func parseRSS(rss *RSS) (*ParsedFeed, []*ParsedArticle, error) {
	feed := &ParsedFeed{Title: cleanText(rss.Channel.Title), Description: cleanText(rss.Channel.Description), SiteURL: rss.Channel.Link}
	if rss.Channel.Image != nil {
		feed.ImageURL = rss.Channel.Image.URL
	}
	var articles []*ParsedArticle
	for _, item := range rss.Channel.Items {
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}
		if guid == "" {
			guid = generateGUID(item.Title + item.Description)
		}
		author := item.Author
		if author == "" {
			author = item.Creator
		}
		imageURL := ""
		if item.Enclosure.URL != "" && strings.HasPrefix(item.Enclosure.Type, "image/") {
			imageURL = item.Enclosure.URL
		}
		articles = append(articles, &ParsedArticle{GUID: guid, Title: cleanText(item.Title), Description: cleanHTML(item.Description), Content: cleanHTML(item.Content), Link: item.Link, Author: author, ImageURL: imageURL, PublishedAt: parsePubDate(item.PubDate)})
	}
	return feed, articles, nil
}

func parseAtom(atom *Atom) (*ParsedFeed, []*ParsedArticle, error) {
	siteURL := ""
	for _, link := range atom.Link {
		if link.Rel == "alternate" || link.Rel == "" {
			siteURL = link.Href
			break
		}
	}
	feed := &ParsedFeed{Title: cleanText(atom.Title), SiteURL: siteURL, ImageURL: atom.Icon}
	var articles []*ParsedArticle
	for _, entry := range atom.Entries {
		link := ""
		for _, l := range entry.Link {
			if l.Rel == "alternate" || l.Rel == "" {
				link = l.Href
				break
			}
		}
		guid := entry.ID
		if guid == "" {
			guid = link
		}
		if guid == "" {
			guid = generateGUID(entry.Title)
		}
		content := entry.Content
		if content == "" {
			content = entry.Summary
		}
		articles = append(articles, &ParsedArticle{GUID: guid, Title: cleanText(entry.Title), Description: cleanHTML(entry.Summary), Content: cleanHTML(content), Link: link, Author: entry.Author.Name, PublishedAt: parseAtomDate(entry.Updated)})
	}
	return feed, articles, nil
}

func parsePubDate(dateStr string) time.Time {
	formats := []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, "Mon, 02 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 02 Jan 2006 15:04:05 MST", "Mon, 2 Jan 2006 15:04:05 MST"}
	dateStr = strings.TrimSpace(dateStr)
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}
	return time.Now()
}

func parseAtomDate(dateStr string) time.Time {
	formats := []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05Z", "2006-01-02"}
	dateStr = strings.TrimSpace(dateStr)
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}
	return time.Now()
}

func cleanText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

func cleanHTML(html string) string {
	html = removeTag(html, "script")
	html = removeTag(html, "style")
	inTag := false
	var result strings.Builder
	for _, char := range html {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(char)
		}
	}
	cleaned := result.String()
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	cleaned = strings.ReplaceAll(cleaned, "&quot;", "\"")
	cleaned = strings.ReplaceAll(cleaned, "&#39;", "'")
	return cleanText(cleaned)
}

func removeTag(html, tag string) string {
	for {
		start := strings.Index(strings.ToLower(html), "<"+tag)
		if start == -1 {
			break
		}
		end := strings.Index(html[start:], "</"+tag+">")
		if end == -1 {
			end = strings.Index(html[start:], "/>")
			if end == -1 {
				break
			}
			end += 2
		} else {
			end += len("</" + tag + ">")
		}
		html = html[:start] + html[start+end:]
	}
	return html
}

func generateGUID(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:16])
}
