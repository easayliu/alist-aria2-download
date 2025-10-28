package alist

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/ratelimit"
	httputil "github.com/easayliu/alist-aria2-download/pkg/httpclient"
)

// Client Alist客户端
type Client struct {
	BaseURL      string
	Username     string
	Password     string
	Token        string
	TokenExpiry  time.Time // Token过期时间
	httpClient   *http.Client
	rateLimiter  *ratelimit.RateLimiter
	tokenMutex   sync.RWMutex // 保护token的读写
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
		Token    string `json:"token"`
		ExpireAt string `json:"expire_at,omitempty"` // Token过期时间(可选)
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

	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()
	
	c.Token = loginResp.Data.Token
	
	// 设置token过期时间，默认1小时后过期
	if loginResp.Data.ExpireAt != "" {
		if expiry, err := time.Parse(time.RFC3339, loginResp.Data.ExpireAt); err == nil {
			c.TokenExpiry = expiry
		} else {
			// 如果解析失败，设置默认过期时间
			c.TokenExpiry = time.Now().Add(1 * time.Hour)
		}
	} else {
		// 没有过期时间信息，设置默认1小时过期
		c.TokenExpiry = time.Now().Add(1 * time.Hour)
	}
	
	return nil
}

// isTokenValid 检查token是否有效（未过期且不为空）
func (c *Client) isTokenValid() bool {
	c.tokenMutex.RLock()
	defer c.tokenMutex.RUnlock()
	
	return c.Token != "" && time.Now().Before(c.TokenExpiry)
}

// ensureValidToken 确保token有效，如果无效则重新登录
func (c *Client) ensureValidToken(ctx context.Context) error {
	if c.isTokenValid() {
		return nil
	}
	
	// token无效，需要重新登录
	return c.LoginWithContext(ctx)
}

// ClearToken 清除当前token，强制下次请求重新登录
func (c *Client) ClearToken() {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()
	
	c.Token = ""
	c.TokenExpiry = time.Time{}
}

// GetTokenStatus 获取token状态信息
func (c *Client) GetTokenStatus() (hasToken bool, isValid bool, expiryTime time.Time) {
	c.tokenMutex.RLock()
	defer c.tokenMutex.RUnlock()
	
	hasToken = c.Token != ""
	isValid = hasToken && time.Now().Before(c.TokenExpiry)
	expiryTime = c.TokenExpiry
	
	return
}

// makeRequest 发起带认证的HTTP请求
func (c *Client) makeRequest(method, endpoint string, reqBody, respBody any) error {
	return c.makeRequestWithContext(context.Background(), method, endpoint, reqBody, respBody)
}

// makeRequestWithContext 发起带认证的HTTP请求（带上下文）
func (c *Client) makeRequestWithContext(ctx context.Context, method, endpoint string, reqBody, respBody any) error {
	// 确保token有效
	if err := c.ensureValidToken(ctx); err != nil {
		return fmt.Errorf("failed to ensure valid token: %w", err)
	}

	// 等待速率限制
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	// 获取当前有效的token
	c.tokenMutex.RLock()
	token := c.Token
	c.tokenMutex.RUnlock()

	// 使用通用HTTP客户端
	opts := httputil.DefaultOptions().
		WithContext(ctx).
		WithClient(c.httpClient)

	if token != "" {
		opts = opts.WithHeader("Authorization", token)
	}

	err := httputil.DoJSONRequest(method, c.BaseURL+endpoint, reqBody, respBody, opts)
	
	// 如果是认证错误，尝试重新登录后再试一次
	if err != nil && isAuthError(err) {
		// 强制重新登录
		c.tokenMutex.Lock()
		c.Token = ""
		c.TokenExpiry = time.Time{}
		c.tokenMutex.Unlock()
		
		// 重新获取token
		if loginErr := c.ensureValidToken(ctx); loginErr != nil {
			return fmt.Errorf("failed to refresh token after auth error: %w", loginErr)
		}
		
		// 获取新token重试请求
		c.tokenMutex.RLock()
		newToken := c.Token
		c.tokenMutex.RUnlock()
		
		if newToken != "" {
			opts = opts.WithHeader("Authorization", newToken)
		}
		
		err = httputil.DoJSONRequest(method, c.BaseURL+endpoint, reqBody, respBody, opts)
	}
	
	return err
}

// isAuthError 判断是否为认证错误
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "401") ||
		   strings.Contains(errStr, "unauthorized") ||
		   strings.Contains(errStr, "invalid token") ||
		   strings.Contains(errStr, "token expired") ||
		   strings.Contains(errStr, "token is invalidated") ||
		   strings.Contains(errStr, "invalidated")
}

// ListFiles 获取文件列表
func (c *Client) ListFiles(path string, page, perPage int) (*FileListResponse, error) {
	return c.ListFilesWithContext(context.Background(), path, page, perPage)
}

// ListFilesWithContext 获取文件列表（带上下文和自动重试）
func (c *Client) ListFilesWithContext(ctx context.Context, path string, page, perPage int) (*FileListResponse, error) {
	// 构建请求参数
	reqData := FileListRequest{
		Path:    path,
		Page:    page,
		PerPage: perPage,
		Refresh: true,
	}

	// 发送请求
	var listResp FileListResponse
	if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/list", reqData, &listResp); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 检查响应状态，如果是认证错误则清除token并重试一次
	if listResp.Code == 401 {
		// 清除token
		c.ClearToken()

		// 重新获取token并重试
		if err := c.ensureValidToken(ctx); err != nil {
			return nil, fmt.Errorf("failed to refresh token after 401: %w", err)
		}

		// 重试请求
		if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/list", reqData, &listResp); err != nil {
			return nil, fmt.Errorf("failed to send request after token refresh: %w", err)
		}
	}

	// 再次检查响应状态
	if listResp.Code != 200 && listResp.Code != 0 {
		return nil, fmt.Errorf("list files failed: code=%d, message=%s", listResp.Code, listResp.Message)
	}

	return &listResp, nil
}

// GetFileInfo 获取文件信息
func (c *Client) GetFileInfo(path string) (*FileGetResponse, error) {
	return c.GetFileInfoWithContext(context.Background(), path)
}

// GetFileInfoWithContext 获取文件信息（带上下文和自动重试）
func (c *Client) GetFileInfoWithContext(ctx context.Context, path string) (*FileGetResponse, error) {
	// 构建请求参数
	reqData := FileGetRequest{
		Path: path,
	}

	// 发送请求
	var getResp FileGetResponse
	if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/get", reqData, &getResp); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 检查响应状态，如果是认证错误则清除token并重试一次
	if getResp.Code == 401 {
		// 清除token
		c.ClearToken()

		// 重新获取token并重试
		if err := c.ensureValidToken(ctx); err != nil {
			return nil, fmt.Errorf("failed to refresh token after 401: %w", err)
		}

		// 重试请求
		if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/get", reqData, &getResp); err != nil {
			return nil, fmt.Errorf("failed to send request after token refresh: %w", err)
		}
	}

	// 再次检查响应状态
	if getResp.Code != 200 && getResp.Code != 0 {
		return nil, fmt.Errorf("get file info failed: code=%d, message=%s", getResp.Code, getResp.Message)
	}

	return &getResp, nil
}

func (c *Client) Rename(path, newName string) error {
	return c.RenameWithContext(context.Background(), path, newName)
}

func (c *Client) RenameWithContext(ctx context.Context, path, newName string) error {
	reqData := RenameRequest{
		Path: path,
		Name: newName,
	}

	var renameResp RenameResponse
	if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/rename", reqData, &renameResp); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	if renameResp.Code == 401 {
		c.ClearToken()

		if err := c.ensureValidToken(ctx); err != nil {
			return fmt.Errorf("failed to refresh token after 401: %w", err)
		}

		if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/rename", reqData, &renameResp); err != nil {
			return fmt.Errorf("failed to send request after token refresh: %w", err)
		}
	}

	if renameResp.Code != 200 && renameResp.Code != 0 {
		return fmt.Errorf("rename failed: code=%d, message=%s", renameResp.Code, renameResp.Message)
	}

	return nil
}

func (c *Client) Move(ctx context.Context, srcDir, dstDir string, names []string) error {
	reqData := MoveRequest{
		SrcDir: srcDir,
		DstDir: dstDir,
		Names:  names,
		Overwrite: true,
	}

	var moveResp MoveResponse
	if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/move", reqData, &moveResp); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	if moveResp.Code == 401 {
		c.ClearToken()

		if err := c.ensureValidToken(ctx); err != nil {
			return fmt.Errorf("failed to refresh token after 401: %w", err)
		}

		if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/move", reqData, &moveResp); err != nil {
			return fmt.Errorf("failed to send request after token refresh: %w", err)
		}
	}

	if moveResp.Code != 200 && moveResp.Code != 0 {
		return fmt.Errorf("move failed: code=%d, message=%s", moveResp.Code, moveResp.Message)
	}

	return nil
}

func (c *Client) Mkdir(ctx context.Context, path string) error {
	reqData := MkdirRequest{
		Path: path,
	}

	var mkdirResp MkdirResponse
	if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/mkdir", reqData, &mkdirResp); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	if mkdirResp.Code == 401 {
		c.ClearToken()

		if err := c.ensureValidToken(ctx); err != nil {
			return fmt.Errorf("failed to refresh token after 401: %w", err)
		}

		if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/mkdir", reqData, &mkdirResp); err != nil {
			return fmt.Errorf("failed to send request after token refresh: %w", err)
		}
	}

	if mkdirResp.Code != 200 && mkdirResp.Code != 0 {
		return fmt.Errorf("mkdir failed: code=%d, message=%s", mkdirResp.Code, mkdirResp.Message)
	}

	return nil
}

func (c *Client) Remove(ctx context.Context, dir string, names []string) error {
	reqData := RemoveRequest{
		Dir:   dir,
		Names: names,
	}

	var removeResp RemoveResponse
	if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/remove", reqData, &removeResp); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	if removeResp.Code == 401 {
		c.ClearToken()

		if err := c.ensureValidToken(ctx); err != nil {
			return fmt.Errorf("failed to refresh token after 401: %w", err)
		}

		if err := c.makeRequestWithContext(ctx, "POST", "/api/fs/remove", reqData, &removeResp); err != nil {
			return fmt.Errorf("failed to send request after token refresh: %w", err)
		}
	}

	if removeResp.Code != 200 && removeResp.Code != 0 {
		return fmt.Errorf("remove failed: code=%d, message=%s", removeResp.Code, removeResp.Message)
	}

	return nil
}
