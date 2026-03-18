package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version"
)

const (
	usageAPIURL       = "https://api.anthropic.com/api/oauth/usage"
	cacheTTL          = 60 * time.Second
	apiTimeout        = 5 * time.Second
	defaultRetryAfter = 60 * time.Second
)

const fileCacheVersion = 1

// fileCacheEntry wraps UsageLimits with metadata for the shared file cache.
type fileCacheEntry struct {
	Version   int          `json:"version"`
	FetchedAt time.Time    `json:"fetched_at"`
	Limits    *UsageLimits `json:"limits"`
}

// Client fetches usage limits from the Claude API.
type Client struct {
	httpClient *http.Client

	cache     *UsageLimits
	cacheTime time.Time
	cacheLock sync.RWMutex

	lastGood *UsageLimits // Preserved for graceful degradation

	fileCachePath string           // Path to shared file cache; empty disables file cache
	nowFunc       func() time.Time // Injectable clock for testing
}

// NewClient creates a new usage API client with default timeout.
func NewClient() *Client {
	return NewClientWithTimeout(apiTimeout)
}

// NewClientWithTimeout creates a new usage API client with configurable timeout.
// Uses a custom Transport with connection pool settings to prevent stale connections
// during long-running sessions.
func NewClientWithTimeout(timeout time.Duration) *Client {
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30 * time.Second,
		ForceAttemptHTTP2:   true,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &Client{
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		fileCachePath: defaultFileCachePath(),
		nowFunc:       time.Now,
	}
}

// defaultFileCachePath returns ~/.cache/cclv/usage.json, or empty string if
// the home directory cannot be determined (disabling file cache gracefully).
func defaultFileCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "cclv", "usage.json")
}

// getCached returns cached UsageLimits if still valid, nil otherwise.
func (c *Client) getCached() *UsageLimits {
	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()

	if c.cache == nil {
		return nil
	}
	if c.nowFunc().Sub(c.cacheTime) > cacheTTL {
		return nil
	}
	return c.cache
}

// setCache stores the result in cache with current timestamp.
func (c *Client) setCache(limits *UsageLimits) {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	c.cache = limits
	c.cacheTime = c.nowFunc()
	c.lastGood = limits
}

// InvalidateCache clears both in-memory and file caches, forcing the next
// FetchUsage call to make a fresh API request. File cache deletion errors
// are silently ignored.
func (c *Client) InvalidateCache() {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	c.cache = nil
	c.cacheTime = time.Time{}

	// Also remove file cache so readFileCache doesn't immediately re-populate
	if c.fileCachePath != "" {
		os.Remove(c.fileCachePath)
	}
}

// readFileCache reads the shared file cache and returns the cached limits if
// fresh and valid. Returns nil on any error or staleness — silent degradation.
func (c *Client) readFileCache() *UsageLimits {
	if c.fileCachePath == "" {
		return nil
	}
	data, err := os.ReadFile(c.fileCachePath)
	if err != nil {
		return nil
	}
	var entry fileCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	if entry.Version != fileCacheVersion {
		return nil
	}
	if entry.Limits == nil {
		return nil
	}
	if c.nowFunc().Sub(entry.FetchedAt) > cacheTTL {
		return nil
	}
	return entry.Limits
}

// writeFileCache atomically writes the usage limits to the shared file cache.
// Uses CreateTemp for unique temp names (safe under concurrent writers) and
// restrictive permissions (0700 dir, 0600 file). Cleans up temp file on failure.
// Silently ignores all errors — file cache is an optimization, not a requirement.
func (c *Client) writeFileCache(limits *UsageLimits) {
	if c.fileCachePath == "" {
		return
	}
	entry := fileCacheEntry{
		Version:   fileCacheVersion,
		FetchedAt: c.nowFunc(),
		Limits:    limits,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	dir := filepath.Dir(c.fileCachePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	tmpFile, err := os.CreateTemp(dir, "usage-*.tmp")
	if err != nil {
		return
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		return
	}
	if err := os.Rename(tmpPath, c.fileCachePath); err != nil {
		os.Remove(tmpPath)
	}
}

// FetchUsage retrieves usage limits, using cache if available.
// Returns (limits, stale, error) where stale=true means returned from lastGood.
func (c *Client) FetchUsage(ctx context.Context, token string) (*UsageLimits, bool, error) {
	// Check in-memory cache first
	if cached := c.getCached(); cached != nil {
		return cached, false, nil
	}

	// Check shared file cache
	if fileCached := c.readFileCache(); fileCached != nil {
		c.setCache(fileCached)
		return fileCached, false, nil
	}

	// Make API request
	limits, err := c.makeRequest(ctx, token)
	if err != nil {
		// On error, return lastGood if available
		c.cacheLock.RLock()
		lastGood := c.lastGood
		c.cacheLock.RUnlock()

		if lastGood != nil {
			return lastGood, true, err
		}
		return nil, false, err
	}

	// Update cache and lastGood on success
	c.setCache(limits)
	c.writeFileCache(limits)
	return limits, false, nil
}

// makeRequest performs the HTTP request to the usage API.
func (c *Client) makeRequest(ctx context.Context, token string) (*UsageLimits, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "cclv/"+version.Version)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Check for context timeout/cancellation
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ErrAPITimeout
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, fmt.Errorf("request canceled: %w", err)
		}
		// Check for HTTP client timeout (url.Error wrapping timeout)
		if isTimeoutError(err) {
			return nil, ErrAPITimeout
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle HTTP status errors
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrTokenExpired
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &RateLimitError{RetryAfter: retryAfter}
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: HTTP %d", ErrAPIError, resp.StatusCode)
	}

	// Parse response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var limits UsageLimits
	if err := json.Unmarshal(body, &limits); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &limits, nil
}

// parseRetryAfter parses the Retry-After header value as delay-seconds (integer).
// Note: RFC 9110 also allows HTTP-date format, but APIs typically use seconds.
// HTTP-date values fall back to defaultRetryAfter.
// Returns defaultRetryAfter if the header is empty, unparseable, or non-positive.
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return defaultRetryAfter
	}
	seconds, err := strconv.Atoi(header)
	if err != nil || seconds <= 0 {
		return defaultRetryAfter
	}
	return time.Duration(seconds) * time.Second
}

// isTimeoutError checks if the error is a timeout error from the HTTP client.
func isTimeoutError(err error) bool {
	type timeoutError interface {
		Timeout() bool
	}
	var te timeoutError
	return errors.As(err, &te) && te.Timeout()
}
