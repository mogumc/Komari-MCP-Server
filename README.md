# Komari-MCP-Server

Go MCP server for Komari server monitoring API. Supports stdio and HTTP transport modes.

> [!IMPORTANT]
> 
> **安全警告**：当配置 `KOMARI_API_KEY` 后，此 MCP 服务器会向 AI 大模型返回**更详细的节点详情（包括节点名、IP 地址、端口等敏感信息）**。  
> 
> 请确保 MCP 客户端的连接对象可信，避免在不受信任或公共的环境中使用 API Key。

## 构建

```bash
# 开发构建
go build -o komari-mcp.exe ./cmd/server

# 带版本号构建（通过 ldflags 注入）
go build -ldflags "-X main.version=1.2.0" -o komari-mcp.exe ./cmd/server
```

## 部署模式

### 模式一：本地 Stdio（默认）

适用于直接集成到本地 MCP 客户端（如 WorkBuddy、Claude Desktop）。

```bash
export KOMARI_BASE_URL="https://your-komari-domain.com"
export KOMARI_API_KEY="your-api-key"

./komari-mcp
```

在 `mcp.json` 中添加：

```json
{
  "mcpServers": {
    "komari": {
      "command": "C:\\path\\to\\komari-mcp.exe",
      "env": {
        "KOMARI_BASE_URL": "https://your-komari-domain.com",
        "KOMARI_API_KEY": "your-api-key"
      }
    }
  }
}
```

### 模式二：远程 HTTP（云端部署）

适用于 MCP 客户端连接远程部署的 MCP 服务器，支持：
- **HTTP POST**：发送 JSON-RPC 请求
- **WebSocket**：双向实时通信

```bash
export KOMARI_BASE_URL="https://your-komari-domain.com"
export KOMARI_API_KEY="your-api-key"
export KOMARI_TRANSPORT="http"
export KOMARI_HTTP_ADDR=":8080"

./komari-mcp
```

服务端提供以下端点：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/mcp` | POST | 接收 JSON-RPC 请求（支持 `application/json` 和 `application/jsonl`） |
| `/ws` | WebSocket | 双向实时通信 |
| `/health` | GET | 健康检查 |

### 模式三：Docker 部署

```bash
docker build -t komari-mcp .
docker run -e KOMARI_BASE_URL=https://... -e KOMARI_API_KEY=... -p 8080:8080 komari-mcp
```

## 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `KOMARI_BASE_URL` | 是 | Komari 服务器地址，如 `https://komari.example.com` |
| `KOMARI_API_KEY` | 是 | API Key 认证（Bearer Token） |
| `KOMARI_TRANSPORT` | 否 | 传输模式：`stdio`（默认）或 `http` |
| `KOMARI_HTTP_ADDR` | 否 | HTTP 监听地址，默认 `:8080` |

## MCP 客户端配置（HTTP）

### WorkBuddy / Claude Desktop

```json
{
  "mcpServers": {
    "komari-remote": {
      "url": "http://your-server:8080/mcp",
      "transport": "streamable-http"
    }
  }
}
```

### 其他支持 HTTP 的客户端

直接通过 HTTP POST 调用：

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "komari_get_version",
      "arguments": {}
    },
    "id": 1
  }'
```

## MCP Tools

| Tool | 说明 | 认证 |
|------|------|------|
| `komari_get_public_info` | 站点公开配置（站点名称、主题、CORS 等） | 无需 |
| `komari_get_version` | 服务端版本号和构建哈希 | 无需 |
| `komari_get_nodes` | 所有/指定节点信息（名称、CPU、内存、磁盘、OS 等），支持 uuid 或 name | 无 API Key 时隐藏节点 |
| `komari_get_latest_status` | 节点实时状态（CPU/内存/磁盘/网络/在线状态），支持 uuid/name/uuids/names | 无 API Key 时隐藏节点 |
| `komari_get_recent_status` | 节点最近约 1 分钟内的状态记录列表，支持 uuid 或 name | 无 API Key 时隐藏节点 |
| `komari_get_records` | 历史监控记录（负载数据或 Ping 延迟），支持 uuid 或 name | 无 API Key 时隐藏节点 |
| `komari_get_full_nodes_status` | 节点完整状态（静态信息 + 实时状态），支持 uuid 或 name | 无 API Key 时隐藏节点 |

## 数据单位

- 内存/磁盘：bytes → `/ 1024^3` = GB
- 网络速度：bytes/s → `* 8 / 1e6` = Mbps
- CPU/负载：百分比
