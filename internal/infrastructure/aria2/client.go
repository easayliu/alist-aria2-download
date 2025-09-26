package aria2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client Aria2客户端
type Client struct {
	RpcURL     string
	Token      string
	httpClient *http.Client
}

// NewClient 创建新的Aria2客户端
func NewClient(rpcURL, token string) *Client {
	return &Client{
		RpcURL: rpcURL,
		Token:  token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RPCRequest JSON-RPC请求结构
type RPCRequest struct {
	Version string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	ID      string        `json:"id"`
	Params  []interface{} `json:"params"`
}

// RPCResponse JSON-RPC响应结构
type RPCResponse struct {
	Version string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError JSON-RPC错误结构
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// AddURIResult 添加URI的响应结果
type AddURIResult string

// StatusResult 状态查询结果
type StatusResult struct {
	GID             string `json:"gid"`
	Status          string `json:"status"`
	TotalLength     string `json:"totalLength"`
	CompletedLength string `json:"completedLength"`
	DownloadSpeed   string `json:"downloadSpeed"`
	ErrorCode       string `json:"errorCode,omitempty"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
	Files           []struct {
		Path string `json:"path"`
		URI  []struct {
			URI    string `json:"uri"`
			Status string `json:"status"`
		} `json:"uris"`
	} `json:"files,omitempty"`
}

// VersionResult 版本信息结果
type VersionResult struct {
	Version  string   `json:"version"`
	Features []string `json:"enabledFeatures"`
}

// callRPC 调用RPC方法
func (c *Client) callRPC(method string, params []interface{}) (*RPCResponse, error) {
	// 如果有token，添加到参数前面
	if c.Token != "" {
		params = append([]interface{}{"token:" + c.Token}, params...)
	}

	request := RPCRequest{
		Version: "2.0",
		Method:  method,
		ID:      fmt.Sprintf("%d", time.Now().Unix()),
		Params:  params,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.RpcURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	return &rpcResp, nil
}

// AddURI 添加下载任务
func (c *Client) AddURI(uri string, options map[string]interface{}) (string, error) {
	params := []interface{}{[]string{uri}}

	if options != nil {
		params = append(params, options)
	}

	resp, err := c.callRPC("aria2.addUri", params)
	if err != nil {
		return "", err
	}

	var gid string
	if err := json.Unmarshal(resp.Result, &gid); err != nil {
		return "", fmt.Errorf("failed to parse GID: %w", err)
	}

	return gid, nil
}

// AddURIs 批量添加下载任务
func (c *Client) AddURIs(uris []string, options map[string]interface{}) ([]string, error) {
	var gids []string

	for _, uri := range uris {
		gid, err := c.AddURI(uri, options)
		if err != nil {
			// 记录错误但继续处理其他URL
			gids = append(gids, fmt.Sprintf("error:%s", err.Error()))
		} else {
			gids = append(gids, gid)
		}
	}

	return gids, nil
}

// GetStatus 获取下载状态
func (c *Client) GetStatus(gid string) (*StatusResult, error) {
	params := []interface{}{gid}

	resp, err := c.callRPC("aria2.tellStatus", params)
	if err != nil {
		return nil, err
	}

	var status StatusResult
	if err := json.Unmarshal(resp.Result, &status); err != nil {
		return nil, fmt.Errorf("failed to parse status: %w", err)
	}

	return &status, nil
}

// GetGlobalStat 获取全局统计信息
func (c *Client) GetGlobalStat() (map[string]interface{}, error) {
	resp, err := c.callRPC("aria2.getGlobalStat", []interface{}{})
	if err != nil {
		return nil, err
	}

	var stat map[string]interface{}
	if err := json.Unmarshal(resp.Result, &stat); err != nil {
		return nil, fmt.Errorf("failed to parse global stat: %w", err)
	}

	return stat, nil
}

// Pause 暂停下载
func (c *Client) Pause(gid string) error {
	_, err := c.callRPC("aria2.pause", []interface{}{gid})
	return err
}

// Resume 恢复下载
func (c *Client) Resume(gid string) error {
	_, err := c.callRPC("aria2.unpause", []interface{}{gid})
	return err
}

// Remove 删除下载
func (c *Client) Remove(gid string) error {
	_, err := c.callRPC("aria2.remove", []interface{}{gid})
	return err
}

// GetVersion 获取Aria2版本信息
func (c *Client) GetVersion() (*VersionResult, error) {
	resp, err := c.callRPC("aria2.getVersion", []interface{}{})
	if err != nil {
		return nil, err
	}

	var version VersionResult
	if err := json.Unmarshal(resp.Result, &version); err != nil {
		return nil, fmt.Errorf("failed to parse version: %w", err)
	}

	return &version, nil
}

// GetActive 获取活动中的下载
func (c *Client) GetActive() ([]StatusResult, error) {
	resp, err := c.callRPC("aria2.tellActive", []interface{}{})
	if err != nil {
		return nil, err
	}

	var active []StatusResult
	if err := json.Unmarshal(resp.Result, &active); err != nil {
		return nil, fmt.Errorf("failed to parse active downloads: %w", err)
	}

	return active, nil
}

// GetWaiting 获取等待中的下载
func (c *Client) GetWaiting(offset, num int) ([]StatusResult, error) {
	resp, err := c.callRPC("aria2.tellWaiting", []interface{}{offset, num})
	if err != nil {
		return nil, err
	}

	var waiting []StatusResult
	if err := json.Unmarshal(resp.Result, &waiting); err != nil {
		return nil, fmt.Errorf("failed to parse waiting downloads: %w", err)
	}

	return waiting, nil
}

// GetStopped 获取已停止的下载
func (c *Client) GetStopped(offset, num int) ([]StatusResult, error) {
	resp, err := c.callRPC("aria2.tellStopped", []interface{}{offset, num})
	if err != nil {
		return nil, err
	}

	var stopped []StatusResult
	if err := json.Unmarshal(resp.Result, &stopped); err != nil {
		return nil, fmt.Errorf("failed to parse stopped downloads: %w", err)
	}

	return stopped, nil
}
