package komari

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mogumc/komari-mcp-server/internal/rpc2"
)

// Client wraps all Komari API interactions.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	APIKey     string
}

// NewClient creates a Komari client with API Key authentication.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// headers builds request headers with auth.
func (c *Client) headers() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		h.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}
	return h
}

// call dispatches a JSON-RPC 2.0 request and returns raw JSON bytes (or error).
func (c *Client) call(method string, params any) ([]byte, error) {
	reqBody, err := json.Marshal(rpc2.Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := strings.TrimSuffix(c.BaseURL, "/") + "/api/rpc2"
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header = c.headers()

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var rpcResp rpc2.Response
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	raw, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return raw, nil
}

// ──────────────────────────────────────────────────────────────────
// Public endpoints (no auth required)
// ──────────────────────────────────────────────────────────────────

// GetPublicInfo returns site public settings.
func (c *Client) GetPublicInfo() (*rpc2.PublicInfo, error) {
	raw, err := c.call("common:getPublicInfo", nil)
	if err != nil {
		return nil, err
	}
	var info rpc2.PublicInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &info, nil
}

// GetVersion returns server version info.
func (c *Client) GetVersion() (*rpc2.VersionInfo, error) {
	raw, err := c.call("common:getVersion", nil)
	if err != nil {
		return nil, err
	}
	var v rpc2.VersionInfo
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &v, nil
}

// ──────────────────────────────────────────────────────────────────
// 合并端点
// ──────────────────────────────────────────────────────────────────

// GetNodesWithStatus 合并 getNodes + getNodesLatestStatus，一次调用获取节点信息和实时状态。
// 这样就可以透过节点名称获取到节点信息和实时状态，避免了两次调用。
func (c *Client) GetNodesWithStatus(uuid string) (map[string]rpc2.NodeWithStatus, error) {

	type nodesResult struct {
		nodes map[string]rpc2.NodeInfo
		err   error
	}
	type statusResult struct {
		status map[string]rpc2.NodeStatus
		err    error
	}

	chNodes := make(chan nodesResult, 1)
	chStatus := make(chan statusResult, 1)

	go func() {
		nodes, err := c.GetNodes(uuid)
		chNodes <- nodesResult{nodes, err}
	}()
	go func() {
		status, err := c.GetNodesLatestStatus(uuid, nil)
		chStatus <- statusResult{status, err}
	}()

	nr := <-chNodes
	sr := <-chStatus

	// 任一失败则返回错误
	if nr.err != nil {
		return nil, fmt.Errorf("getNodes: %w", nr.err)
	}
	if sr.err != nil {
		return nil, fmt.Errorf("getNodesLatestStatus: %w", sr.err)
	}

	// 合并结果
	result := make(map[string]rpc2.NodeWithStatus, len(nr.nodes))
	for id, info := range nr.nodes {
		nws := rpc2.NodeWithStatus{NodeInfo: info}
		if s, ok := sr.status[id]; ok {
			nws.Status = &s
		}
		result[id] = nws
	}
	return result, nil
}

// ──────────────────────────────────────────────────────────────────
// Authenticated endpoints
// ──────────────────────────────────────────────────────────────────

// GetNodes returns all nodes (or one by uuid).
func (c *Client) GetNodes(uuid string) (map[string]rpc2.NodeInfo, error) {
	params := map[string]string{}
	if uuid != "" {
		params["uuid"] = uuid
	}
	raw, err := c.call("common:getNodes", params)
	if err != nil {
		return nil, err
	}

	var result map[string]rpc2.NodeInfo
	if err := json.Unmarshal(raw, &result); err == nil && len(result) > 0 {
		return result, nil
	}

	if uuid != "" {
		var single rpc2.NodeInfo
		if err := json.Unmarshal(raw, &single); err != nil {
			return nil, fmt.Errorf("unmarshal single node: %w", err)
		}
		return map[string]rpc2.NodeInfo{uuid: single}, nil
	}
	return nil, fmt.Errorf("unexpected getNodes response format")
}

// GetNodesLatestStatus returns latest status for one or more nodes.
func (c *Client) GetNodesLatestStatus(uuid string, uuids []string) (map[string]rpc2.NodeStatus, error) {
	params := map[string]any{}
	if uuid != "" {
		params["uuid"] = uuid
	}
	if len(uuids) > 0 {
		params["uuids"] = uuids
	}
	raw, err := c.call("common:getNodesLatestStatus", params)
	if err != nil {
		return nil, err
	}

	var result map[string]rpc2.NodeStatus
	if err := json.Unmarshal(raw, &result); err == nil && len(result) > 0 {
		return result, nil
	}

	if uuid != "" {
		var single rpc2.NodeStatus
		if err := json.Unmarshal(raw, &single); err != nil {
			return nil, fmt.Errorf("unmarshal single node status: %w", err)
		}
		return map[string]rpc2.NodeStatus{uuid: single}, nil
	}
	return nil, fmt.Errorf("unexpected getNodesLatestStatus response format")
}

// GetNodeRecentStatus returns last ~1 minute of status records.
func (c *Client) GetNodeRecentStatus(uuid string) (*rpc2.RecentStatusResp, error) {
	params := map[string]string{"uuid": uuid}
	raw, err := c.call("common:getNodeRecentStatus", params)
	if err != nil {
		return nil, err
	}
	var r rpc2.RecentStatusResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &r, nil
}

// GetRecords returns historical load or ping records.
func (c *Client) GetRecords(recordType, uuid string, hours, maxCount int, loadType, taskID string, start, end string) (*rpc2.RecordsResponse, error) {
	params := map[string]any{
		"type": recordType,
	}
	if uuid != "" {
		params["uuid"] = uuid
	}
	// start/end 优先于 hours
	if start != "" || end != "" {
		if start != "" {
			params["start"] = start
		}
		if end != "" {
			params["end"] = end
		}
	} else {
		if hours <= 0 {
			hours = 1
		}
		params["hours"] = hours
	}
	if loadType != "" {
		params["load_type"] = loadType
	}
	if taskID != "" {
		params["task_id"] = taskID
	}
	if maxCount != 0 {
		params["maxCount"] = maxCount
	}
	raw, err := c.call("common:getRecords", params)
	if err != nil {
		return nil, err
	}
	var r rpc2.RecordsResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &r, nil
}
