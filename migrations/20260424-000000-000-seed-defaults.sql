INSERT OR IGNORE INTO feeds (id, url, title, created_at, updated_at) VALUES
    (hex(randomblob(16)), 'https://hnrss.org/frontpage', 'Hacker News', strftime('%s','now'), strftime('%s','now')),
    (hex(randomblob(16)), 'https://www.theverge.com/rss/index.xml', 'The Verge', strftime('%s','now'), strftime('%s','now')),
    (hex(randomblob(16)), 'https://feeds.arstechnica.com/arstechnica/index', 'Ars Technica', strftime('%s','now'), strftime('%s','now')),
    (hex(randomblob(16)), 'https://blog.golang.org/feed.atom', 'Go Blog', strftime('%s','now'), strftime('%s','now')),
    (hex(randomblob(16)), 'https://www.wired.com/feed/rss', 'WIRED', strftime('%s','now'), strftime('%s','now')),
    (hex(randomblob(16)), 'https://feeds.bbci.co.uk/news/world/rss.xml', 'BBC World News', strftime('%s','now'), strftime('%s','now')),
    (hex(randomblob(16)), 'https://rss.nytimes.com/services/xml/rss/nyt/Technology.xml', 'NYT Technology', strftime('%s','now'), strftime('%s','now'));
