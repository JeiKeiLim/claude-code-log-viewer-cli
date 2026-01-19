package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/JeiKeiLim/claude-code-log-viewer-cli/internal/version"
)

const (
	usageAPIURL = "https://api.anthropic.com/api/oauth/usage"
	cacheTTL    = 60 * time.Second
	apiTimeout  = 5 * time.Second
)

// Client fetches usage limits from the Claude API.
type Client struct {
	httpClient *http.Client

	cache     *UsageLimits
	cacheTime time.Time
	cacheLock sync.RWMutex

	lastGood *UsageLimits // Preserved for graceful degradation
}

// NewClient creates a new usage API client with default timeout.
func NewClient() *Client {
	return NewClientWithTimeout(apiTimeout)
}

// NewClientWithTimeout creates a new usage API client with configurable timeout.
func NewClientWithTimeout(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

// getCached returns cached UsageLimits if still valid, nil otherwise.
func (c *Client) getCached() *UsageLimits {
	c.cacheLock.RLock()
	defer c.cacheLock.RUnlock()

	if c.cache == nil {
		return nil
	}
	if time.Since(c.cacheTime) > cacheTTL {
		return nil
	}
	return c.cache
}

// setCache stores the result in cache with current timestamp.
func (c *Client) setCache(limits *UsageLimits) {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	c.cache = limits
	c.cacheTime = time.Now()
	c.lastGood = limits
}

// InvalidateCache clears the cache, forcing the next FetchUsage call to make an API request.
func (c *Client) InvalidateCache() {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	c.cache = nil
	c.cacheTime = time.Time{}
}

// FetchUsage retrieves usage limits, using cache if available.
// Returns (limits, stale, error) where stale=true means returned from lastGood.
func (c *Client) FetchUsage(ctx context.Context, token string) (*UsageLimits, bool, error) {
	// Check cache first
	if cached := c.getCached(); cached != nil {
		return cached, false, nil
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
	req.Header.Set("User-Agent", "claude-code/cclv-"+version.Version)
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
	defer resp.Body.Close()

	// Handle HTTP status errors
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrTokenExpired
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

// isTimeoutError checks if the error is a timeout error from the HTTP client.
func isTimeoutError(err error) bool {
	type timeoutError interface {
		Timeout() bool
	}
	var te timeoutError
	return errors.As(err, &te) && te.Timeout()
}
