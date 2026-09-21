package updatecheck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		current, latest string
		available       bool
	}{
		{"0.2.1", "v0.2.2", true},
		{"1.9.9", "v2.0.0", true},
		{"v1.2.3", "1.2.3", false},
		{"1.2.4", "v1.2.3", false},
		{"1.2.3-beta.1", "v1.2.3", true},
		{"1.2.3", "v1.2.3-beta.2", false},
	}
	for _, tt := range tests {
		current, ok := parseVersion(tt.current)
		if !ok {
			t.Fatalf("parse current %q", tt.current)
		}
		got := resultFor(current, tt.current, cachedRelease{TagName: tt.latest}).Available
		if got != tt.available {
			t.Errorf("%s -> %s available=%v, want %v", tt.current, tt.latest, got, tt.available)
		}
	}
	for _, invalid := range []string{"", "dev", "1", "1.2", "01.2.3", "1.2.x", "1.2.3-"} {
		if _, ok := parseVersion(invalid); ok {
			t.Errorf("parseVersion(%q) unexpectedly succeeded", invalid)
		}
	}
}

func TestCheckFetchesAndCachesLatestRelease(t *testing.T) {
	now := time.Date(2026, time.September, 21, 12, 0, 0, 0, time.UTC)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("User-Agent") != "taskii-update-check" {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v0.4.0", HTMLURL: "https://example.test/release"})
	}))
	defer server.Close()

	cachePath := filepath.Join(t.TempDir(), "nested", "update-check.json")
	checker := Checker{
		Endpoint:  server.URL,
		CachePath: cachePath,
		Client:    server.Client(),
		Now:       func() time.Time { return now },
		TTL:       24 * time.Hour,
	}
	first, err := checker.Check("0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Available || first.Current != "0.3.0" || first.Latest != "0.4.0" || first.URL != "https://example.test/release" {
		t.Fatalf("unexpected result: %+v", first)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache was not written: %v", err)
	}

	second, err := checker.Check("0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Available || requests != 1 {
		t.Fatalf("fresh cache not used: result=%+v requests=%d", second, requests)
	}
}

func TestCheckUsesStaleCacheWhenOffline(t *testing.T) {
	now := time.Date(2026, time.September, 21, 12, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(t.TempDir(), "update-check.json")
	cached := cachedRelease{TagName: "v0.5.0", HTMLURL: "https://example.test/v0.5.0", CheckedAt: now.Add(-48 * time.Hour)}
	b, _ := json.Marshal(cached)
	if err := os.WriteFile(cachePath, b, 0o600); err != nil {
		t.Fatal(err)
	}

	checker := Checker{
		Endpoint:  "http://127.0.0.1:1",
		CachePath: cachePath,
		Client:    &http.Client{Timeout: 50 * time.Millisecond},
		Now:       func() time.Time { return now },
		TTL:       24 * time.Hour,
	}
	got, err := checker.Check("0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.Latest != "0.5.0" {
		t.Fatalf("stale fallback = %+v", got)
	}
}

func TestCheckRejectsInvalidVersionsAndResponses(t *testing.T) {
	checker := Checker{CachePath: filepath.Join(t.TempDir(), "cache")}
	if _, err := checker.Check("dev"); err == nil {
		t.Fatal("dev version should not be checked")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"latest"}`))
	}))
	defer server.Close()
	checker.Endpoint = server.URL
	checker.Client = server.Client()
	if _, err := checker.Check("0.3.0"); err == nil {
		t.Fatal("invalid release tag should fail")
	}
}
