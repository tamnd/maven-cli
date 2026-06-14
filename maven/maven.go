// Package maven is the library behind the mvn command line:
// the HTTP client, request shaping, and the typed data models for Maven Central.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package maven

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://search.maven.org"

// DefaultUserAgent identifies the client to Maven Central.
const DefaultUserAgent = "maven-cli/0.1.0 (github.com/tamnd/maven-cli)"

// Config holds constructor parameters.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   defaultBaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   5,
		Timeout:   30 * time.Second,
	}
}

// Client talks to Maven Central over HTTP.
type Client struct {
	http      *http.Client
	userAgent string
	baseURL   string
	rate      time.Duration
	retries   int
	last      time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		http:      &http.Client{Timeout: cfg.Timeout},
		userAgent: cfg.UserAgent,
		baseURL:   strings.TrimRight(base, "/"),
		rate:      cfg.Rate,
		retries:   cfg.Retries,
	}
}

// Get fetches rawURL and returns the response body.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

func (c *Client) getJSON(ctx context.Context, rawURL string, v any) error {
	body, err := c.Get(ctx, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

// ─── wire types ──────────────────────────────────────────────────────────────

type solrResponse struct {
	ResponseHeader struct {
		Status int `json:"status"`
	} `json:"responseHeader"`
	Response struct {
		NumFound int       `json:"numFound"`
		Start    int       `json:"start"`
		Docs     []solrDoc `json:"docs"`
	} `json:"response"`
}

type solrDoc struct {
	ID             string `json:"id"`
	G              string `json:"g"`
	A              string `json:"a"`
	LatestVersion  string `json:"latestVersion"`
	V              string `json:"v"`
	P              string `json:"p"`
	Timestamp      int64  `json:"timestamp"`
	VersionCount   int    `json:"versionCount"`
}

// ─── Search ──────────────────────────────────────────────────────────────────

// Search queries Maven Central and returns up to limit artifacts.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Artifact, error) {
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("rows", fmt.Sprintf("%d", limit))
	params.Set("wt", "json")

	rawURL := c.baseURL + "/solrsearch/select?" + params.Encode()
	var resp solrResponse
	if err := c.getJSON(ctx, rawURL, &resp); err != nil {
		return nil, err
	}

	out := make([]Artifact, 0, len(resp.Response.Docs))
	for _, d := range resp.Response.Docs {
		out = append(out, docToArtifact(d))
	}
	return out, nil
}

// Artifact fetches the latest info for a specific groupId:artifactId.
func (c *Client) Artifact(ctx context.Context, groupID, artifactID string) (Artifact, error) {
	q := fmt.Sprintf("g:%s+AND+a:%s", groupID, artifactID)
	params := url.Values{}
	params.Set("q", q)
	params.Set("rows", "1")
	params.Set("wt", "json")

	rawURL := c.baseURL + "/solrsearch/select?" + params.Encode()
	var resp solrResponse
	if err := c.getJSON(ctx, rawURL, &resp); err != nil {
		return Artifact{}, err
	}
	if len(resp.Response.Docs) == 0 {
		return Artifact{}, fmt.Errorf("artifact %s:%s not found", groupID, artifactID)
	}
	return docToArtifact(resp.Response.Docs[0]), nil
}

// Versions lists available versions for a specific groupId:artifactId.
func (c *Client) Versions(ctx context.Context, groupID, artifactID string, limit int) ([]Version, error) {
	if limit <= 0 {
		limit = 10
	}
	q := fmt.Sprintf("g:%s+AND+a:%s", groupID, artifactID)
	params := url.Values{}
	params.Set("q", q)
	params.Set("core", "gav")
	params.Set("rows", fmt.Sprintf("%d", limit))
	params.Set("wt", "json")

	rawURL := c.baseURL + "/solrsearch/select?" + params.Encode()
	var resp solrResponse
	if err := c.getJSON(ctx, rawURL, &resp); err != nil {
		return nil, err
	}

	out := make([]Version, 0, len(resp.Response.Docs))
	for _, d := range resp.Response.Docs {
		out = append(out, docToVersion(d, groupID, artifactID))
	}
	return out, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func artifactURL(groupID, artifactID string) string {
	return "https://central.sonatype.com/artifact/" + groupID + "/" + artifactID
}

func fmtTimestamp(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format("2006-01-02")
}

func docToArtifact(d solrDoc) Artifact {
	g := d.G
	a := d.A
	if g == "" || a == "" {
		// parse from id field "g:a"
		parts := strings.SplitN(d.ID, ":", 2)
		if len(parts) == 2 {
			g = parts[0]
			a = parts[1]
		}
	}
	return Artifact{
		GroupID:       g,
		ArtifactID:    a,
		LatestVersion: d.LatestVersion,
		Packaging:     d.P,
		LastUpdated:   fmtTimestamp(d.Timestamp),
		VersionCount:  d.VersionCount,
		URL:           artifactURL(g, a),
	}
}

func docToVersion(d solrDoc, groupID, artifactID string) Version {
	return Version{
		Version:     d.V,
		LastUpdated: fmtTimestamp(d.Timestamp),
		URL:         artifactURL(groupID, artifactID) + "/" + d.V,
	}
}
