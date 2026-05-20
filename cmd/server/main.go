package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/mogumc/komari-mcp-server/internal/komari"
)

// ──────────────────────────────────────────────────────────────────────────────
// Transport Mode
// ──────────────────────────────────────────────────────────────────────────────

type TransportMode string

const (
	ModeStdio TransportMode = "stdio"
	ModeHTTP  TransportMode = "http"
)

// ──────────────────────────────────────────────────────────────────────────────
// MCP Types
// ──────────────────────────────────────────────────────────────────────────────

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

type ToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      any             `json:"id,omitempty"`
}

type MCPResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
	ID      any    `json:"id,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Tool Definitions
// ──────────────────────────────────────────────────────────────────────────────

var toolList = []Tool{
	{
		Name:        "komari_get_public_info",
		Description: "获取 Komari 站点的公开配置（站点名称、主题、CORS 设置等）。无需认证，始终可用。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		Name:        "komari_get_version",
		Description: "获取 Komari 服务端版本号和构建哈希。无需认证，始终可用。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		Name:        "komari_get_nodes",
		Description: "获取所有节点或指定节点的信息（名称、CPU、内存、磁盘、OS 等）。无 API Key 时隐藏节点和敏感字段会被过滤。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uuid": map[string]any{"type": "string", "description": "节点 UUID，留空获取全部"},
				"name": map[string]any{"type": "string", "description": "节点名称，用于按名称筛选节点（优先级低于 uuid）"},
			},
		},
	},
	{
		Name:        "komari_get_latest_status",
		Description: "获取一个或多个节点的最新实时状态（CPU/内存/磁盘/网络速度/在线状态）。无 API Key 时隐藏节点被过滤。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uuid":  map[string]any{"type": "string", "description": "单个节点 UUID"},
				"uuids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "多个节点 UUID 列表"},
				"name":  map[string]any{"type": "string", "description": "单个节点名称，按名称查询实时状态（优先级低于 uuid）"},
				"names": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "多个节点名称列表，按名称批量查询实时状态（优先级低于 uuids）"},
			},
		},
	},
	{
		Name:        "komari_get_recent_status",
		Description: "获取指定节点最近约 1 分钟内的状态记录列表。无 API Key 时隐藏节点被过滤。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uuid": map[string]any{"type": "string", "description": "节点 UUID（必填，与 name 二选一）"},
				"name": map[string]any{"type": "string", "description": "节点名称（必填，与 uuid 二选一）"},
			},
		},
	},
	{
		Name:        "komari_get_records",
		Description: "获取节点的历史监控记录（负载数据或 Ping 延迟）。支持通过 hours 或 start/end 指定时间范围。无 API Key 时隐藏节点被过滤。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"type":      map[string]any{"type": "string", "enum": []any{"load", "ping"}, "description": "记录类型：load 或 ping"},
				"uuid":      map[string]any{"type": "string", "description": "节点 UUID，留空表示全部"},
				"name":      map[string]any{"type": "string", "description": "节点名称，按名称查询历史记录（优先级低于 uuid）"},
				"hours":     map[string]any{"type": "integer", "description": "时间范围（小时），默认 1。与 start/end 互斥"},
				"start":     map[string]any{"type": "string", "description": "起始时间 RFC3339（如 2026-01-01T00:00:00Z），与 hours 互斥"},
				"end":       map[string]any{"type": "string", "description": "结束时间 RFC3339，缺省为当前时间"},
				"load_type": map[string]any{"type": "string", "enum": []any{"cpu", "gpu", "ram", "swap", "load", "temp", "disk", "network", "process", "connections", "all"}, "description": "仅 type=load 时有效，指定指标类型"},
				"task_id":   map[string]any{"type": "integer", "description": "仅 type=ping 时有效，指定任务 ID，-1 或留空表示全部"},
				"max_count": map[string]any{"type": "integer", "description": "数据点上限，默认 4000，-1 表示不限"},
			},
			"required": []any{"type"},
		},
	},
	{
		Name:        "komari_get_full_nodes_status",
		Description: "获取节点的完整状态：静态信息（名称、CPU、内存、OS 等）+ 实时状态（CPU 使用率、在线状态等）。通过 name 或 uuid 可筛选单个节点，留空获取全部。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uuid": map[string]any{"type": "string", "description": "节点 UUID，留空获取全部"},
				"name": map[string]any{"type": "string", "description": "节点名称，用于按名称筛选节点（优先级低于 uuid）"},
			},
		},
	},
}

// ──────────────────────────────────────────────────────────────────────────────
// MCP Server
// ──────────────────────────────────────────────────────────────────────────────

type MCPServer struct {
	client    *komari.Client
	tools     []Tool
	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex
	broadcast chan MCPResponse
}

func NewMCPServer(client *komari.Client) *MCPServer {
	return &MCPServer{
		client:    client,
		tools:     toolList,
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan MCPResponse, 100),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Transport: Stdio
// ──────────────────────────────────────────────────────────────────────────────

func (s *MCPServer) runStdio() {
	dec := json.NewDecoder(os.Stdin)
	for dec.More() {
		var req MCPRequest
		if err := dec.Decode(&req); err != nil {
			if err != io.EOF {
				log.Printf("decode error: %v", err)
			}
			continue
		}
		s.stdioHandle(req)
	}
}

func (s *MCPServer) stdioHandle(req MCPRequest) {
	resp := s.processRequest(req)
	if resp == nil {
		return
	}
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Printf("stdio write error: %v", err)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Transport: HTTP
// ──────────────────────────────────────────────────────────────────────────────

func (s *MCPServer) runHTTP(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCP)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 启动 WebSocket 广播协程
	go s.wsBroadcaster()

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		srv.Shutdown(context.Background())
	}()

	log.Printf("MCP HTTP server listening on %s", addr)
	return srv.ListenAndServe()
}

func (s *MCPServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *MCPServer) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/json" {
		var req MCPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendHTTPError(w, nil, -32700, "Parse error")
			return
		}
		resp := s.processRequest(req)
		if resp != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	} else if contentType == "application/jsonl" || contentType == "application/jsonl+batch" {
		var responses []MCPResponse
		dec := json.NewDecoder(r.Body)
		for dec.More() {
			var req MCPRequest
			if err := dec.Decode(&req); err == nil {
				if resp := s.processRequest(req); resp != nil {
					responses = append(responses, *resp)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	} else {
		http.Error(w, "Unsupported content type", http.StatusUnsupportedMediaType)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// WebSocket
// ──────────────────────────────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *MCPServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()
	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var req MCPRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}

		if resp := s.processRequest(req); resp != nil {
			conn.WriteJSON(resp)
		}
	}
}

func (s *MCPServer) wsBroadcaster() {
	for resp := range s.broadcast {
		s.clientsMu.Lock()
		for conn := range s.clients {
			if err := conn.WriteJSON(resp); err != nil {
				conn.Close()
				delete(s.clients, conn)
			}
		}
		s.clientsMu.Unlock()
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Request Processing
// ──────────────────────────────────────────────────────────────────────────────

func (s *MCPServer) processRequest(req MCPRequest) *MCPResponse {
	var resp MCPResponse
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	switch req.Method {
	case "initialize":
		resp.Result = handleInitialize()

	case "tools/list":
		resp.Result = map[string]any{"tools": s.tools}

	case "tools/call":
		var tc ToolCall
		if err := json.Unmarshal(req.Params, &tc); err != nil {
			resp.Error = map[string]any{"code": -32600, "message": "Invalid request: " + err.Error()}
			return &resp
		}
		result, err := s.callTool(tc.Name, tc.Arguments)
		if err != nil {
			resp.Error = map[string]any{"code": -32603, "message": err.Error()}
			return &resp
		}
		resp.Result = result

	case "ping":
		resp.Result = map[string]string{"pong": "ok"}

	case "notifications/initialized", "notifications/stopped":
		return nil

	default:
		resp.Error = map[string]any{"code": -32601, "message": fmt.Sprintf("method not found: %s", req.Method)}
	}

	return &resp
}

func (s *MCPServer) sendHTTPError(w http.ResponseWriter, req *MCPRequest, code int, message string) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		Error:   map[string]any{"code": code, "message": message},
	}
	if req != nil {
		resp.ID = req.ID
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// ──────────────────────────────────────────────────────────────────────────────
// Tool Router
// ──────────────────────────────────────────────────────────────────────────────

func (s *MCPServer) callTool(name string, rawArgs json.RawMessage) (any, error) {
	switch name {
	case "komari_get_public_info":
		info, err := s.client.GetPublicInfo()
		if err != nil {
			return nil, err
		}
		return textContent(info), nil

	case "komari_get_version":
		v, err := s.client.GetVersion()
		if err != nil {
			return nil, err
		}
		return textContent(v), nil

	case "komari_get_nodes":
		var args struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		}
		if len(rawArgs) > 0 && json.Unmarshal(rawArgs, &args) != nil {
			return nil, fmt.Errorf("invalid arguments")
		}
		uuid := args.UUID
		if uuid == "" && args.Name != "" {
			resolved, err := s.client.ResolveNameToUUID(args.Name)
			if err != nil {
				return nil, err
			}
			uuid = resolved
		}
		nodes, err := s.client.GetNodes(uuid)
		if err != nil {
			return nil, err
		}
		return textContent(nodes), nil

	case "komari_get_latest_status":
		var args struct {
			UUID  string   `json:"uuid"`
			UUIDs []string `json:"uuids"`
			Name  string   `json:"name"`
			Names []string `json:"names"`
		}
		if len(rawArgs) > 0 && json.Unmarshal(rawArgs, &args) != nil {
			return nil, fmt.Errorf("invalid arguments")
		}
		uuid := args.UUID
		uuids := args.UUIDs
		// 优先使用 uuid/uuids，如果未提供则尝试从 name/names 解析
		if uuid == "" && len(uuids) == 0 {
			if args.Name != "" {
				resolved, err := s.client.ResolveNameToUUID(args.Name)
				if err != nil {
					return nil, err
				}
				uuid = resolved
			} else if len(args.Names) > 0 {
				resolved, err := s.client.ResolveNamesToUUIDs(args.Names)
				if err != nil {
					return nil, err
				}
				uuids = resolved
			}
		}
		status, err := s.client.GetNodesLatestStatus(uuid, uuids)
		if err != nil {
			return nil, err
		}
		return textContent(status), nil

	case "komari_get_recent_status":
		var args struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		}
		if len(rawArgs) > 0 && json.Unmarshal(rawArgs, &args) != nil {
			return nil, fmt.Errorf("invalid arguments")
		}
		uuid := args.UUID
		if uuid == "" && args.Name != "" {
			resolved, err := s.client.ResolveNameToUUID(args.Name)
			if err != nil {
				return nil, err
			}
			uuid = resolved
		}
		if uuid == "" {
			return nil, fmt.Errorf("uuid 或 name 至少需要提供一个")
		}
		rec, err := s.client.GetNodeRecentStatus(uuid)
		if err != nil {
			return nil, err
		}
		return textContent(rec), nil

	case "komari_get_records":
		var args struct {
			Type     string `json:"type"`
			UUID     string `json:"uuid"`
			Name     string `json:"name"`
			Hours    int    `json:"hours"`
			Start    string `json:"start"`
			End      string `json:"end"`
			LoadType string `json:"load_type"`
			TaskID   int    `json:"task_id"`
			MaxCount int    `json:"max_count"`
		}
		if len(rawArgs) > 0 && json.Unmarshal(rawArgs, &args) != nil {
			return nil, fmt.Errorf("invalid arguments")
		}
		if args.Type == "" {
			return nil, fmt.Errorf("type (load or ping) is required")
		}
		uuid := args.UUID
		if uuid == "" && args.Name != "" {
			resolved, err := s.client.ResolveNameToUUID(args.Name)
			if err != nil {
				return nil, err
			}
			uuid = resolved
		}
		if args.Hours == 0 {
			args.Hours = 1
		}
		// load_type 仅对 type=load 有效
		if args.Type != "load" {
			args.LoadType = ""
		}
		// task_id 仅对 type=ping 有效
		taskIDStr := ""
		if args.Type == "ping" && args.TaskID != 0 {
			taskIDStr = fmt.Sprintf("%d", args.TaskID)
		}
		rec, err := s.client.GetRecords(args.Type, uuid, args.Hours, args.MaxCount, args.LoadType, taskIDStr, args.Start, args.End)
		if err != nil {
			return nil, err
		}
		return textContent(rec), nil

	case "komari_get_full_nodes_status":
		var args struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		}
		if len(rawArgs) > 0 && json.Unmarshal(rawArgs, &args) != nil {
			return nil, fmt.Errorf("invalid arguments")
		}
		uuid := args.UUID
		if uuid == "" && args.Name != "" {
			resolved, err := s.client.ResolveNameToUUID(args.Name)
			if err != nil {
				return nil, err
			}
			uuid = resolved
		}
		result, err := s.client.GetNodesWithStatus(uuid)
		if err != nil {
			return nil, err
		}
		return textContent(result), nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func textContent(v any) any {
	b, _ := json.MarshalIndent(v, "", "  ")
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(b)},
		},
	}
}

func handleInitialize() map[string]any {
	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "komari-mcp",
			"version": version,
		},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// main
// ──────────────────────────────────────────────────────────────────────────────

var version = "dev"

func main() {
	baseURL := os.Getenv("KOMARI_BASE_URL")
	apiKey := os.Getenv("KOMARI_API_KEY")

	if baseURL == "" {
		log.Fatal("KOMARI_BASE_URL environment variable is required")
	}
	// API Key 可选: 公开端点 (getPublicInfo, getVersion, getNodes, getNodesLatestStatus)
	// 无需认证即可访问，仅管理端点需要 token。
	if apiKey == "" {
		log.Println("KOMARI_API_KEY not set — public endpoints only (no admin/client auth)")
	}

	mode := TransportMode(os.Getenv("KOMARI_TRANSPORT"))
	if mode == "" {
		mode = ModeStdio
	}

	addr := os.Getenv("KOMARI_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	client := komari.NewClient(baseURL, apiKey)
	server := NewMCPServer(client)

	if mode == ModeHTTP {
		log.Printf("Starting in HTTP mode on %s", addr)
		if err := server.runHTTP(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	} else {
		log.Println("Starting in stdio mode")
		server.runStdio()
	}
}
