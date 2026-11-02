// Package httpx provides a shared HTTP client with sane timeouts, a
// user agent, and optional in-memory response caching.
package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cmerk2021/panely/internal/appinfo"
)

// DefaultTimeout is the default request timeout.
const DefaultTimeout = 15 * time.Second

// UserAgent sent with every request.
var UserAgent = appinfo.Name + "/" + appinfo.Version

// Client wraps http.Client with a user agent and optional caching.
type Client struct {
	hc    *http.Client
	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	body    []byte
	expires time.Time
}

// NewClient returns a Client with the given timeout (0 means DefaultTimeout).
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		hc:    &http.Client{Timeout: timeout},
		cache: make(map[string]cacheEntry),
	}
}

// Get fetches url and returns the body. Non-2xx responses are errors.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	return c.GetCached(ctx, url, 0)
}

// GetCached fetches url, serving a cached copy if one newer than ttl exists.
// ttl <= 0 disables caching.
func (c *Client) GetCached(ctx context.Context, url string, ttl time.Duration) ([]byte, error) {
	if ttl > 0 {
		c.mu.Lock()
		if e, ok := c.cache[url]; ok && time.Now().Before(e.expires) {
			body := e.body
			c.mu.Unlock()
			return body, nil
		}
		c.mu.Unlock()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	if ttl > 0 {
		c.mu.Lock()
		c.cache[url] = cacheEntry{body: body, expires: time.Now().Add(ttl)}
		c.mu.Unlock()
	}
	return body, nil
}

// Do performs an arbitrary request with the shared client and user agent.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}
	return c.hc.Do(req)
}

// Shared is a process-wide client with the default timeout.
var Shared = NewClient(0)
