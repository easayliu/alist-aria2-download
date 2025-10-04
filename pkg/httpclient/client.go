package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Options HTTP请求选项
type Options struct {
	// 超时时间，默认30秒
	Timeout time.Duration
	// 请求头
	Headers map[string]string
	// 上下文，用于取消请求
	Context context.Context
	// HTTP客户端，如果为nil则使用默认客户端
	Client *http.Client
}

// DefaultOptions 返回默认选项
func DefaultOptions() *Options {
	return &Options{
		Timeout: 30 * time.Second,
		Headers: make(map[string]string),
		Context: context.Background(),
	}
}

// WithTimeout 设置超时时间
func (o *Options) WithTimeout(timeout time.Duration) *Options {
	o.Timeout = timeout
	return o
}

// WithHeader 添加请求头
func (o *Options) WithHeader(key, value string) *Options {
	if o.Headers == nil {
		o.Headers = make(map[string]string)
	}
	o.Headers[key] = value
	return o
}

// WithContext 设置上下文
func (o *Options) WithContext(ctx context.Context) *Options {
	o.Context = ctx
	return o
}

// WithClient 设置HTTP客户端
func (o *Options) WithClient(client *http.Client) *Options {
	o.Client = client
	return o
}

// DoJSONRequest 执行JSON请求，统一处理JSON编码/解码和HTTP请求
func DoJSONRequest(method, url string, reqBody, respBody interface{}, opts ...*Options) error {
	// 获取选项
	var options *Options
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	} else {
		options = DefaultOptions()
	}

	// 获取HTTP客户端
	client := options.Client
	if client == nil {
		client = &http.Client{
			Timeout: options.Timeout,
		}
	}

	// 处理请求体
	var reqReader io.Reader
	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqReader = bytes.NewBuffer(jsonData)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(options.Context, method, url, reqReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置默认Content-Type
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 设置自定义头部
	for key, value := range options.Headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应体
	if respBody != nil {
		if err := json.Unmarshal(body, respBody); err != nil {
			return fmt.Errorf("failed to unmarshal response body: %w", err)
		}
	}

	return nil
}

// PostJSON 发送POST JSON请求的便捷方法
func PostJSON(url string, reqBody, respBody interface{}, opts ...*Options) error {
	return DoJSONRequest("POST", url, reqBody, respBody, opts...)
}

// GetJSON 发送GET JSON请求的便捷方法
func GetJSON(url string, respBody interface{}, opts ...*Options) error {
	return DoJSONRequest("GET", url, nil, respBody, opts...)
}

// PutJSON 发送PUT JSON请求的便捷方法
func PutJSON(url string, reqBody, respBody interface{}, opts ...*Options) error {
	return DoJSONRequest("PUT", url, reqBody, respBody, opts...)
}

// PatchJSON 发送PATCH JSON请求的便捷方法
func PatchJSON(url string, reqBody, respBody interface{}, opts ...*Options) error {
	return DoJSONRequest("PATCH", url, reqBody, respBody, opts...)
}

// DeleteJSON 发送DELETE JSON请求的便捷方法
func DeleteJSON(url string, reqBody, respBody interface{}, opts ...*Options) error {
	return DoJSONRequest("DELETE", url, reqBody, respBody, opts...)
}

// JSONFileUtils 提供JSON文件读写工具
type JSONFileUtils struct{}

// ReadJSONFile 从文件读取JSON数据
func (j *JSONFileUtils) ReadJSONFile(filename string, v interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from file %s: %w", filename, err)
	}

	return nil
}

// WriteJSONFile 将JSON数据写入文件
func (j *JSONFileUtils) WriteJSONFile(filename string, v interface{}, indent bool) error {
	var data []byte
	var err error

	if indent {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filename, err)
	}

	return nil
}

// NewJSONFileUtils 创建JSON文件工具实例
func NewJSONFileUtils() *JSONFileUtils {
	return &JSONFileUtils{}
}