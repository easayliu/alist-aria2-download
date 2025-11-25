package tmdb

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/ratelimit"
	httputil "github.com/easayliu/alist-aria2-download/pkg/httpclient"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

const (
	DefaultBaseURL = "https://api.themoviedb.org/3"
	DefaultTimeout = 10 * time.Second
)

type Client struct {
	BaseURL     string
	APIKey      string
	Language    string
	httpClient  *http.Client
	rateLimiter *ratelimit.RateLimiter
	mu          sync.RWMutex
}

func NewClient(apiKey string) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("TMDB_API_KEY")
	}

	return &Client{
		BaseURL:  DefaultBaseURL,
		APIKey:   apiKey,
		Language: "en-US",
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		rateLimiter: ratelimit.NewRateLimiter(40),
	}
}

func (c *Client) SetLanguage(lang string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Language = lang
}

func (c *Client) SetQPS(qps int) {
	if c.rateLimiter != nil {
		c.rateLimiter.SetQPS(qps)
	}
}

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, params url.Values, result interface{}) error {
	if c.APIKey == "" {
		return fmt.Errorf("TMDB API key is not set")
	}

	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", c.APIKey)

	c.mu.RLock()
	lang := c.Language
	c.mu.RUnlock()

	if lang != "" {
		params.Set("language", lang)
	}

	urlStr := fmt.Sprintf("%s%s?%s", c.BaseURL, endpoint, params.Encode())

	// 添加调试日志
	logger.Debug("TMDB API Request",
		"method", method,
		"endpoint", endpoint,
		"language", lang,
		"url", urlStr)

	opts := httputil.DefaultOptions().
		WithContext(ctx).
		WithClient(c.httpClient)

	err := httputil.DoJSONRequest(method, urlStr, nil, result, opts)
	if err != nil {
		logger.Error("TMDB API Request failed", "url", urlStr, "error", err)
	}
	return err
}

func (c *Client) SearchMovie(ctx context.Context, query string, year int) (*SearchMovieResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("include_adult", "true")
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}

	var resp SearchMovieResponse
	if err := c.makeRequest(ctx, "GET", "/search/movie", params, &resp); err != nil {
		return nil, fmt.Errorf("failed to search movie: %w", err)
	}

	return &resp, nil
}

func (c *Client) SearchTV(ctx context.Context, query string, year int) (*SearchTVResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("include_adult", "true")

	var resp SearchTVResponse
	if err := c.makeRequest(ctx, "GET", "/search/tv", params, &resp); err != nil {
		return nil, fmt.Errorf("failed to search TV: %w", err)
	}

	return &resp, nil
}

func (c *Client) GetMovieDetails(ctx context.Context, movieID int) (*MovieDetails, error) {
	endpoint := fmt.Sprintf("/movie/%d", movieID)

	var details MovieDetails
	if err := c.makeRequest(ctx, "GET", endpoint, nil, &details); err != nil {
		return nil, fmt.Errorf("failed to get movie details: %w", err)
	}

	return &details, nil
}

func (c *Client) GetTVDetails(ctx context.Context, tvID int) (*TVDetails, error) {
	endpoint := fmt.Sprintf("/tv/%d", tvID)

	var details TVDetails
	if err := c.makeRequest(ctx, "GET", endpoint, nil, &details); err != nil {
		return nil, fmt.Errorf("failed to get TV details: %w", err)
	}

	return &details, nil
}

func (c *Client) GetSeasonDetails(ctx context.Context, tvID, seasonNumber int) (*Season, error) {
	endpoint := fmt.Sprintf("/tv/%d/season/%d", tvID, seasonNumber)

	var season Season
	if err := c.makeRequest(ctx, "GET", endpoint, nil, &season); err != nil {
		return nil, fmt.Errorf("failed to get season details: %w", err)
	}

	return &season, nil
}
