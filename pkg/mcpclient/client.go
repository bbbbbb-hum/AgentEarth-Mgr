package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ProtocolVersion = "2024-11-05"
	ClientName      = "ae-mcp-tester"
	ClientVersion   = "1.0.0"
)

// Client MCP客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
	requestID  int
	sessionID  string
}

// NewClient 创建新的MCP客户端
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		requestID: 0,
		sessionID: "",
	}
}

// BuildServiceURL 构建wemcp服务地址
// pattern: "http://ae-{name}-service:8081"
// wemcpName: "wemcp2-qweather"
// result: "http://ae-wemcp2-qweather-service:8081"
func BuildServiceURL(pattern, wemcpName string) string {
	return strings.Replace(pattern, "{name}", wemcpName, 1)
}

// extractJSONFromSSE 从 SSE 格式响应中提取 JSON 数据
// SSE 格式: "event: message\ndata: {json}\n\n"
func extractJSONFromSSE(body string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	// 如果没有找到 data: 前缀，尝试直接作为 JSON 解析
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "{") {
		return body
	}
	return ""
}

// updateSessionID 从响应头更新会话ID
func (c *Client) updateSessionID(header http.Header) {
	if sessionID := header.Get("Mcp-Session-Id"); sessionID != "" {
		c.sessionID = sessionID
	}
}

// Connect 连接并获取工具列表
func (c *Client) Connect(ctx context.Context) (*ConnectResult, error) {
	start := time.Now()
	result := &ConnectResult{Success: true}

	// Step 1: Initialize
	initResult, err := c.Initialize(ctx)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("初始化失败: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}
	result.ServerInfo = &initResult.ServerInfo

	// Step 2: List Tools
	tools, err := c.ListTools(ctx)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("获取工具列表失败: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}
	result.Tools = tools
	result.ToolsCount = len(tools)
	result.DurationMs = time.Since(start).Milliseconds()

	return result, nil
}

// Initialize 发送初始化请求
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    map[string]interface{}{},
		ClientInfo: ClientInfo{
			Name:    ClientName,
			Version: ClientVersion,
		},
	}

	resp, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析初始化响应失败: %v", err)
	}

	// 发送 initialized 通知
	_ = c.sendNotification(ctx, "notifications/initialized", nil)

	return &result, nil
}

// ListTools 获取工具列表
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	resp, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析工具列表响应失败: %v", err)
	}

	return result.Tools, nil
}

// CallTool 调用工具
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*CallResult, error) {
	start := time.Now()
	result := &CallResult{Success: true}

	params := CallToolParams{
		Name:      name,
		Arguments: arguments,
	}

	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	var callResult CallToolResult
	if err := json.Unmarshal(resp.Result, &callResult); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("解析调用结果失败: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	// 只存 content 数组部分，避免双层嵌套
	contentBytes, err := json.Marshal(callResult.Content)
	if err != nil {
		result.Content = resp.Result // fallback: 存完整结果
	} else {
		result.Content = contentBytes
	}
	result.IsError = callResult.IsError
	result.DurationMs = time.Since(start).Milliseconds()

	return result, nil
}

// sendRequest 发送JSON-RPC请求
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	c.requestID++

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.requestID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()
	c.updateSessionID(resp.Header)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP错误: %d, body: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析 SSE 格式响应，提取 data: 行中的 JSON
	jsonData := extractJSONFromSSE(string(respBody))
	if jsonData == "" {
		return nil, fmt.Errorf("无法从SSE响应中提取JSON: %s", string(respBody))
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal([]byte(jsonData), &rpcResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v, body: %s", err, jsonData)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC错误 [%d]: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return &rpcResp, nil
}

// sendNotification 发送JSON-RPC通知（无响应）
func (c *Client) sendNotification(ctx context.Context, method string, params interface{}) error {
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.updateSessionID(resp.Header)

	return nil
}
