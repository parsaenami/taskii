// Package updatecheck checks GitHub Releases for a newer stable Taskii build.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
)

const (
	latestReleaseURL = "https://api.github.com/repos/parsaenami/taskii/releases/latest"
	releasesPageURL  = "https://github.com/parsaenami/taskii/releases/latest"
	cacheFileName    = "taskii/update-check.json"
)

const (
	defaultTTL     = 24 * time.Hour
	requestTimeout = 4 * time.Second
	maxResponse    = 1 << 20
)

// Result is the latest stable release known to the checker.
type Result struct {
	Current   string
	Latest    string
	URL       string
	Available bool
	CheckedAt time.Time
}

// Checker contains injectable dependencies so release and cache behaviour can
// be tested without contacting GitHub or writing to a user's real cache.
type Checker struct {
	Endpoint  string
	CachePath string
	Client    *http.Client
	Now       func() time.Time
	TTL       time.Duration
}

type cachedRelease struct {
	TagName   string    `json:"tag_name"`
	HTMLURL   string    `json:"html_url"`
	CheckedAt time.Time `json:"checked_at"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// New returns the production checker. Failure to resolve an XDG cache path is
// intentionally non-fatal: the network check can still run without caching.
func New() Checker {
	cachePath, _ := xdg.CacheFile(cacheFileName)
	return Checker{
		Endpoint:  latestReleaseURL,
		CachePath: cachePath,
		Client:    &http.Client{Timeout: requestTimeout},
		Now:       time.Now,
		TTL:       defaultTTL,
	}
}

// Check returns the latest release, using a fresh cache entry when possible.
// A stale valid entry is returned as a fallback when GitHub is unavailable.
func (c Checker) Check(current string) (Result, error) {
	currentVersion, ok := parseVersion(current)
	if !ok {
		return Result{}, fmt.Errorf("invalid current version %q", current)
	}
	c = c.withDefaults()
	now := c.Now()

	stale, haveStale := c.readCache()
	if haveStale {
		age := now.Sub(stale.CheckedAt)
		if age >= 0 && age <= c.TTL {
			return resultFor(currentVersion, current, stale), nil
		}
	}

	release, err := c.fetch(now)
	if err != nil {
		if haveStale {
			return resultFor(currentVersion, current, stale), nil
		}
		return Result{}, err
	}
	_ = c.writeCache(release)
	return resultFor(currentVersion, current, release), nil
}

func (c Checker) withDefaults() Checker {
	if c.Endpoint == "" {
		c.Endpoint = latestReleaseURL
	}
	if c.Client == nil {
		c.Client = &http.Client{Timeout: requestTimeout}
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.TTL <= 0 {
		c.TTL = defaultTTL
	}
	return c
}

func (c Checker) fetch(now time.Time) (cachedRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint, nil)
	if err != nil {
		return cachedRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "taskii-update-check")

	resp, err := c.Client.Do(req)
	if err != nil {
		return cachedRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return cachedRelease{}, fmt.Errorf("GitHub releases returned %s", resp.Status)
	}

	var release githubRelease
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxResponse))
	if err := dec.Decode(&release); err != nil {
		return cachedRelease{}, fmt.Errorf("decode release: %w", err)
	}
	if _, ok := parseVersion(release.TagName); !ok {
		return cachedRelease{}, fmt.Errorf("invalid release tag %q", release.TagName)
	}
	if release.HTMLURL == "" {
		release.HTMLURL = releasesPageURL
	}
	return cachedRelease{TagName: release.TagName, HTMLURL: release.HTMLURL, CheckedAt: now}, nil
}

func (c Checker) readCache() (cachedRelease, bool) {
	if c.CachePath == "" {
		return cachedRelease{}, false
	}
	b, err := os.ReadFile(c.CachePath)
	if err != nil {
		return cachedRelease{}, false
	}
	var cached cachedRelease
	if json.Unmarshal(b, &cached) != nil || cached.CheckedAt.IsZero() {
		return cachedRelease{}, false
	}
	if _, ok := parseVersion(cached.TagName); !ok {
		return cachedRelease{}, false
	}
	if cached.HTMLURL == "" {
		cached.HTMLURL = releasesPageURL
	}
	return cached, true
}

func (c Checker) writeCache(cached cachedRelease) error {
	if c.CachePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.CachePath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.CachePath), ".update-check-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, c.CachePath)
}

func resultFor(current parsedVersion, currentText string, release cachedRelease) Result {
	latest, ok := parseVersion(release.TagName)
	if !ok {
		return Result{Current: normalizeVersion(currentText), CheckedAt: release.CheckedAt}
	}
	return Result{
		Current:   normalizeVersion(currentText),
		Latest:    normalizeVersion(release.TagName),
		URL:       release.HTMLURL,
		Available: compareVersions(latest, current) > 0,
		CheckedAt: release.CheckedAt,
	}
}

type parsedVersion struct {
	major, minor, patch uint64
	prerelease          []string
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// parseVersion implements the comparison-relevant portion of Semantic
// Versioning. Build metadata is ignored; prerelease identifiers follow SemVer
// numeric/alphanumeric precedence rules.
func parseVersion(input string) (parsedVersion, bool) {
	v := normalizeVersion(input)
	if v == "" {
		return parsedVersion{}, false
	}
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	var prerelease []string
	if i := strings.IndexByte(v, '-'); i >= 0 {
		pre := v[i+1:]
		v = v[:i]
		if pre == "" {
			return parsedVersion{}, false
		}
		prerelease = strings.Split(pre, ".")
		for _, part := range prerelease {
			if part == "" {
				return parsedVersion{}, false
			}
		}
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return parsedVersion{}, false
	}
	values := make([]uint64, 3)
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return parsedVersion{}, false
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return parsedVersion{}, false
		}
		values[i] = n
	}
	return parsedVersion{major: values[0], minor: values[1], patch: values[2], prerelease: prerelease}, true
}

func compareVersions(a, b parsedVersion) int {
	for _, pair := range [][2]uint64{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(a.prerelease) == 0 && len(b.prerelease) == 0 {
		return 0
	}
	if len(a.prerelease) == 0 {
		return 1
	}
	if len(b.prerelease) == 0 {
		return -1
	}
	for i := 0; i < len(a.prerelease) && i < len(b.prerelease); i++ {
		left, right := a.prerelease[i], b.prerelease[i]
		leftNum, leftErr := strconv.ParseUint(left, 10, 64)
		rightNum, rightErr := strconv.ParseUint(right, 10, 64)
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNum < rightNum {
				return -1
			}
			if leftNum > rightNum {
				return 1
			}
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		default:
			if left < right {
				return -1
			}
			if left > right {
				return 1
			}
		}
	}
	if len(a.prerelease) < len(b.prerelease) {
		return -1
	}
	if len(a.prerelease) > len(b.prerelease) {
		return 1
	}
	return 0
}
