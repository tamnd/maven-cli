package maven_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/maven-cli/maven"
)

func testClient(ts *httptest.Server) *maven.Client {
	cfg := maven.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return maven.NewClient(cfg)
}

func TestGetSendsUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte(`"hello"`))
	}))
	defer srv.Close()

	c := testClient(srv)
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `"hello"` {
		t.Errorf("body = %q", body)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`"recovered"`))
	}))
	defer srv.Close()

	cfg := maven.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := maven.NewClient(cfg)

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `"recovered"` {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func solrResp(docs []map[string]any) []byte {
	resp := map[string]any{
		"responseHeader": map[string]any{"status": 0},
		"response": map[string]any{
			"numFound": len(docs),
			"start":    0,
			"docs":     docs,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestSearch(t *testing.T) {
	docs := []map[string]any{
		{
			"id":            "org.junit.jupiter:junit-jupiter",
			"g":             "org.junit.jupiter",
			"a":             "junit-jupiter",
			"latestVersion": "5.10.2",
			"p":             "jar",
			"timestamp":     int64(1711152000000),
			"versionCount":  10,
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(solrResp(docs))
	}))
	defer srv.Close()

	c := testClient(srv)
	results, err := c.Search(context.Background(), "junit", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	a := results[0]
	if a.GroupID != "org.junit.jupiter" {
		t.Errorf("GroupID = %q", a.GroupID)
	}
	if a.ArtifactID != "junit-jupiter" {
		t.Errorf("ArtifactID = %q", a.ArtifactID)
	}
	if a.LatestVersion != "5.10.2" {
		t.Errorf("LatestVersion = %q", a.LatestVersion)
	}
	if a.VersionCount != 10 {
		t.Errorf("VersionCount = %d", a.VersionCount)
	}
	if a.LastUpdated != "2024-03-23" {
		t.Errorf("LastUpdated = %q", a.LastUpdated)
	}
	if a.URL == "" {
		t.Error("URL is empty")
	}
}

func TestArtifact(t *testing.T) {
	docs := []map[string]any{
		{
			"id":            "com.example:mylib",
			"g":             "com.example",
			"a":             "mylib",
			"latestVersion": "1.2.3",
			"p":             "jar",
			"timestamp":     int64(1700000000000),
			"versionCount":  3,
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(solrResp(docs))
	}))
	defer srv.Close()

	c := testClient(srv)
	a, err := c.Artifact(context.Background(), "com.example", "mylib")
	if err != nil {
		t.Fatal(err)
	}
	if a.ArtifactID != "mylib" {
		t.Errorf("ArtifactID = %q", a.ArtifactID)
	}
	if a.LatestVersion != "1.2.3" {
		t.Errorf("LatestVersion = %q", a.LatestVersion)
	}
}

func TestArtifactNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(solrResp(nil))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.Artifact(context.Background(), "com.missing", "nope")
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}

func TestVersions(t *testing.T) {
	docs := []map[string]any{
		{"g": "org.junit.jupiter", "a": "junit-jupiter", "v": "5.10.2", "timestamp": int64(1711152000000)},
		{"g": "org.junit.jupiter", "a": "junit-jupiter", "v": "5.10.1", "timestamp": int64(1700000000000)},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(solrResp(docs))
	}))
	defer srv.Close()

	c := testClient(srv)
	versions, err := c.Versions(context.Background(), "org.junit.jupiter", "junit-jupiter", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}
	if versions[0].Version != "5.10.2" {
		t.Errorf("version[0] = %q", versions[0].Version)
	}
	if versions[0].URL == "" {
		t.Error("URL is empty")
	}
}
