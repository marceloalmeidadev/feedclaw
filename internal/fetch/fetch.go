// Package fetch implements the concurrent feed fetcher: conditional requests,
// byte limits, an always-on SSRF guard and per-feed error accounting.
package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/marceloalmeidadev/feedclaw/internal/store"
	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/errgroup"
)

// Version is stamped into the User-Agent; overridden at build time via ldflags.
var Version = "dev"

const repoURL = "https://github.com/marceloalmeidadev/feedclaw"

// Config tunes the fetcher. Zero values fall back to safe defaults.
type Config struct {
	Workers        int
	ConnectTimeout time.Duration
	RequestTimeout time.Duration
	MaxFeedBytes   int64
	SecurityMode   SecurityMode
	AllowedHosts   []string
}

func (c Config) withDefaults() Config {
	if c.Workers <= 0 {
		c.Workers = 8
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = 10 * time.Second
	}
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = 30 * time.Second
	}
	if c.MaxFeedBytes <= 0 {
		c.MaxFeedBytes = 2 << 20 // 2 MiB
	}
	return c
}

// Fetcher fetches and stores feeds.
type Fetcher struct {
	cfg    Config
	store  *store.Store
	guard  *Guard
	client *http.Client
	parser *gofeed.Parser
}

// New builds a Fetcher bound to a store.
func New(st *store.Store, cfg Config) *Fetcher {
	cfg = cfg.withDefaults()
	guard := NewGuard(cfg.SecurityMode, cfg.AllowedHosts)
	transport := &http.Transport{
		DialContext:           guard.safeDialContext(cfg.ConnectTimeout),
		TLSHandshakeTimeout:   cfg.ConnectTimeout,
		ResponseHeaderTimeout: cfg.RequestTimeout,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}
	client := &http.Client{
		Timeout:       cfg.RequestTimeout,
		Transport:     transport,
		CheckRedirect: guard.checkRedirect,
	}
	return &Fetcher{
		cfg:    cfg,
		store:  st,
		guard:  guard,
		client: client,
		parser: gofeed.NewParser(),
	}
}

// Result summarizes the outcome of fetching a single feed.
type Result struct {
	FeedID      int64  `json:"feed_id"`
	FeedURL     string `json:"feed_url"`
	Status      int    `json:"status"`
	NotModified bool   `json:"not_modified"`
	NewArticles int    `json:"new_articles"`
	Err         string `json:"error,omitempty"`
}

// Run fetches the given feeds concurrently. If feeds is nil all enabled feeds
// are fetched. It returns one Result per feed; individual failures do not abort
// the batch.
func (f *Fetcher) Run(ctx context.Context, feeds []*store.Feed) ([]Result, error) {
	if feeds == nil {
		all, err := f.store.ListFeeds()
		if err != nil {
			return nil, err
		}
		for _, feed := range all {
			if !feed.Disabled {
				feeds = append(feeds, feed)
			}
		}
	}

	results := make([]Result, len(feeds))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(f.cfg.Workers)

	for i, feed := range feeds {
		i, feed := i, feed
		g.Go(func() error {
			results[i] = f.fetchAndStore(ctx, feed)
			return nil // errors are captured per-result, never fail the group
		})
	}
	if err := g.Wait(); err != nil {
		return results, err
	}
	return results, nil
}

// fetchAndStore performs one feed's fetch, parse and persistence, updating feed
// health accordingly.
func (f *Fetcher) fetchAndStore(ctx context.Context, feed *store.Feed) Result {
	res := Result{FeedID: feed.ID, FeedURL: feed.URL}

	body, status, etag, lastMod, notModified, err := f.get(ctx, feed.URL, feed.ETag, feed.LastModified)
	res.Status = status
	if err != nil {
		res.Err = err.Error()
		_ = f.store.MarkFetchError(feed.ID, status)
		return res
	}
	if notModified {
		res.NotModified = true
		_ = f.store.MarkFetchSuccess(feed.ID, status, etag, lastMod)
		return res
	}

	parsed, err := f.parser.ParseString(string(body))
	if err != nil {
		res.Err = fmt.Sprintf("parse: %v", err)
		_ = f.store.MarkFetchError(feed.ID, status)
		return res
	}

	for _, item := range parsed.Items {
		art := itemToArticle(feed.ID, item)
		inserted, err := f.store.UpsertArticle(art)
		if err != nil {
			res.Err = fmt.Sprintf("store: %v", err)
			continue
		}
		if inserted {
			res.NewArticles++
		}
	}
	_ = f.store.MarkFetchSuccess(feed.ID, status, etag, lastMod)
	return res
}

// get performs the conditional HTTP GET with byte limiting. It returns the body
// only when the server responds 200; notModified is true for 304.
func (f *Fetcher) get(ctx context.Context, url, etag, lastMod string) (body []byte, status int, newETag, newLastMod string, notModified bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, "", "", false, err
	}
	if err := f.guard.CheckURL(req.URL.Scheme, req.URL.Host); err != nil {
		return nil, 0, "", "", false, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("FeedClaw/%s (+%s)", Version, repoURL))
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, application/json;q=0.9, */*;q=0.5")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastMod != "" {
		req.Header.Set("If-Modified-Since", lastMod)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, 0, "", "", false, err
	}
	defer resp.Body.Close()

	status = resp.StatusCode
	newETag = resp.Header.Get("ETag")
	newLastMod = resp.Header.Get("Last-Modified")

	switch {
	case status == http.StatusNotModified:
		return nil, status, newETag, newLastMod, true, nil
	case status >= 200 && status < 300:
		limited := io.LimitReader(resp.Body, f.cfg.MaxFeedBytes+1)
		data, readErr := io.ReadAll(limited)
		if readErr != nil {
			return nil, status, "", "", false, readErr
		}
		if int64(len(data)) > f.cfg.MaxFeedBytes {
			return nil, status, "", "", false, fmt.Errorf("feed exceeds max_feed_bytes (%d)", f.cfg.MaxFeedBytes)
		}
		return data, status, newETag, newLastMod, false, nil
	default:
		return nil, status, "", "", false, fmt.Errorf("unexpected status %d", status)
	}
}

// itemToArticle maps a gofeed item onto our article model.
func itemToArticle(feedID int64, item *gofeed.Item) *store.Article {
	a := &store.Article{
		FeedID:  feedID,
		GUID:    item.GUID,
		URL:     item.Link,
		Title:   item.Title,
		Summary: item.Description,
		Content: item.Content,
	}
	if len(item.Authors) > 0 && item.Authors[0] != nil {
		a.Author = item.Authors[0].Name
	}
	if item.PublishedParsed != nil {
		t := item.PublishedParsed.UTC()
		a.PublishedAt = &t
	} else if item.UpdatedParsed != nil {
		t := item.UpdatedParsed.UTC()
		a.PublishedAt = &t
	}
	if a.GUID == "" {
		a.GUID = store.FallbackGUID(a.URL, a.Title)
	}
	return a
}
