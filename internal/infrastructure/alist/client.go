package alist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client Alist客户端
type Client struct {
	BaseURL    string
	Username   string
	Password   string
	Token      string
	httpClient *http.Client
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
	}
}

// Login 调用/api/auth/login获取token
func (c *Client) Login() error {
	loginReq := LoginRequest{
		Username: c.Username,
		Password: c.Password,
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/api/auth/login", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if loginResp.Code != 200 {
		return fmt.Errorf("login failed: code=%d, message=%s", loginResp.Code, loginResp.Message)
	}

	c.Token = loginResp.Data.Token
	return nil
}

// makeRequest 发起带认证的HTTP请求
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, c.BaseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", c.Token)
	}

	return c.httpClient.Do(req)
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
	}

	// 发送请求
	resp, err := c.makeRequest("POST", "/api/fs/list", reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 解析响应
	var listResp FileListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
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
	resp, err := c.makeRequest("POST", "/api/fs/get", reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 解析响应
	var getResp FileGetResponse
	if err := json.Unmarshal(body, &getResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 检查响应状态
	if getResp.Code != 200 && getResp.Code != 0 {
		return nil, fmt.Errorf("get file info failed: code=%d, message=%s", getResp.Code, getResp.Message)
	}

	return &getResp, nil
}