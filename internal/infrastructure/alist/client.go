package alist

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/ratelimit"
	httputil "github.com/easayliu/alist-aria2-download/pkg/http"
)

// Client Alist客户端
type Client struct {
	BaseURL     string
	Username    string
	Password    string
	Token       string
	httpClient  *http.Client
	rateLimiter *ratelimit.RateLimiter
}

// LoginRequest 登录请求结构
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

// NewClient 创建新的Alist客户端
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: ratelimit.NewRateLimiter(50), // 默认QPS为50
	}
}

// NewClientWithQPS 创建带QPS限制的Alist客户端
func NewClientWithQPS(baseURL, username, password string, qps int) *Client {
	return &Client{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: ratelimit.NewRateLimiter(qps),
	}
}

// SetQPS 设置QPS限制
func (c *Client) SetQPS(qps int) {
	if c.rateLimiter != nil {
		c.rateLimiter.SetQPS(qps)
	}
}

// GetQPS 获取当前QPS限制
func (c *Client) GetQPS() int {
	if c.rateLimiter != nil {
		return c.rateLimiter.GetQPS()
	}
	return 0
}

// Login 调用/api/auth/login获取token
func (c *Client) Login() error {
	return c.LoginWithContext(context.Background())
}

// LoginWithContext 调用/api/auth/login获取token（带上下文）
func (c *Client) LoginWithContext(ctx context.Context) error {
	// 等待速率限制
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit exceeded during login: %w", err)
		}
	}

	loginReq := LoginRequest{
		Username: c.Username,
		Password: c.Password,
	}

	// 使用通用HTTP客户端
	opts := httputil.DefaultOptions().
		WithContext(ctx).
		WithClient(c.httpClient)

	var loginResp LoginResponse
	if err := httputil.PostJSON(c.BaseURL+"/api/auth/login", loginReq, &loginResp, opts); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	if loginResp.Code != 200 {
		return fmt.Errorf("login failed: code=%d, message=%s", loginResp.Code, loginResp.Message)
	}

	c.Token = loginResp.Data.Token
	return nil
}

// makeRequest 发起带认证的HTTP请求
func (c *Client) makeRequest(method, endpoint string, reqBody, respBody interface{}) error {
	return c.makeRequestWithContext(context.Background(), method, endpoint, reqBody, respBody)
}

// makeRequestWithContext 发起带认证的HTTP请求（带上下文）
func (c *Client) makeRequestWithContext(ctx context.Context, method, endpoint string, reqBody, respBody interface{}) error {
	// 等待速率限制
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	// 使用通用HTTP客户端
	opts := httputil.DefaultOptions().
		WithContext(ctx).
		WithClient(c.httpClient)

	if c.Token != "" {
		opts = opts.WithHeader("Authorization", c.Token)
	}

	return httputil.DoJSONRequest(method, c.BaseURL+endpoint, reqBody, respBody, opts)
}

// ListFiles 获取文件列表
func (c *Client) ListFiles(path string, page, perPage int) (*FileListResponse, error) {
	// 如果没有token，先登录
	if c.Token == "" {
		if err := c.Login(); err != nil {
			return nil, fmt.Errorf("failed to login: %w", err)
		}
	}

	// 构建请求参数
	reqData := FileListRequest{
		Path:    path,
		Page:    page,
		PerPage: perPage,
		Refresh: true,
	}

	// 发送请求
	var listResp FileListResponse
	if err := c.makeRequest("POST", "/api/fs/list", reqData, &listResp); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 检查响应状态
	if listResp.Code != 200 && listResp.Code != 0 {
		return nil, fmt.Errorf("list files failed: code=%d, message=%s", listResp.Code, listResp.Message)
	}

	return &listResp, nil
}

// GetFileInfo 获取文件信息
func (c *Client) GetFileInfo(path string) (*FileGetResponse, error) {
	// 如果没有token，先登录
	if c.Token == "" {
		if err := c.Login(); err != nil {
			return nil, fmt.Errorf("failed to login: %w", err)
		}
	}

	// 构建请求参数
	reqData := FileGetRequest{
		Path: path,
	}

	// 发送请求
	var getResp FileGetResponse
	if err := c.makeRequest("POST", "/api/fs/get", reqData, &getResp); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 检查响应状态
	if getResp.Code != 200 && getResp.Code != 0 {
		return nil, fmt.Errorf("get file info failed: code=%d, message=%s", getResp.Code, getResp.Message)
	}

	return &getResp, nil
}
