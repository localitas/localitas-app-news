package news

type DefaultFeed struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

var DefaultFeeds = []DefaultFeed{
	{URL: "https://hnrss.org/frontpage", Title: "Hacker News", Category: "Tech", Description: "Top stories from Hacker News"},
	{URL: "https://www.theverge.com/rss/index.xml", Title: "The Verge", Category: "Tech", Description: "Technology, science, art, and culture"},
	{URL: "https://feeds.arstechnica.com/arstechnica/index", Title: "Ars Technica", Category: "Tech", Description: "Technology news and analysis"},
	{URL: "https://blog.golang.org/feed.atom", Title: "Go Blog", Category: "Dev", Description: "Official Go programming blog"},
	{URL: "https://www.wired.com/feed/rss", Title: "WIRED", Category: "Tech", Description: "Technology and culture"},
	{URL: "https://feeds.bbci.co.uk/news/world/rss.xml", Title: "BBC World News", Category: "News", Description: "World news from BBC"},
	{URL: "https://rss.nytimes.com/services/xml/rss/nyt/Technology.xml", Title: "NYT Technology", Category: "Tech", Description: "Technology news from The New York Times"},
}
