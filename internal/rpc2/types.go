package rpc2

// JSON-RPC 2.0 Request
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	ID      any    `json:"id"`
}

// JSON-RPC 2.0 Response
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
	ID      any    `json:"id"`
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ──────────────────────────────────────────────────────────────────
// common:getNodes
// ──────────────────────────────────────────────────────────────────

type NodeInfo struct {
	UUID             string  `json:"uuid"`
	Token            string  `json:"token,omitempty"`
	Name             string  `json:"name"`
	CpuName          string  `json:"cpu_name"`
	Virtualization   string  `json:"virtualization"`
	Arch             string  `json:"arch"`
	CpuCores         int     `json:"cpu_cores"`
	OS               string  `json:"os"`
	KernelVersion    string  `json:"kernel_version"`
	GpuName          string  `json:"gpu_name"`
	IPv4             string  `json:"ipv4,omitempty"`
	IPv6             string  `json:"ipv6,omitempty"`
	Region           string  `json:"region"`
	Remark           string  `json:"remark,omitempty"`
	PublicRemark     string  `json:"public_remark"`
	MemTotal         int64   `json:"mem_total"`
	SwapTotal        int64   `json:"swap_total"`
	DiskTotal        int64   `json:"disk_total"`
	Version          string  `json:"version,omitempty"`
	Weight           int     `json:"weight"`
	Price            float64 `json:"price"`
	BillingCycle     int     `json:"billing_cycle"`
	AutoRenewal      bool    `json:"auto_renewal"`
	Currency         string  `json:"currency"`
	ExpiredAt        string  `json:"expired_at"`
	Group            string  `json:"group"`
	Tags             string  `json:"tags"`
	Hidden           bool    `json:"hidden"`
	TrafficLimit     int64   `json:"traffic_limit"`
	TrafficLimitType string  `json:"traffic_limit_type"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// ──────────────────────────────────────────────────────────────────
// common:getNodes + common:getNodesLatestStatus
// ──────────────────────────────────────────────────────────────────

// NodeWithStatus 合并 NodeInfo 和 NodeStatus
type NodeWithStatus struct {
	NodeInfo
	Status *NodeStatus `json:"status,omitempty"` // nil 表示节点离线或无状态数据
}

// ──────────────────────────────────────────────────────────────────
// common:getNodesLatestStatus
// ──────────────────────────────────────────────────────────────────

type PingResult struct {
	Name   string  `json:"name"`
	Latest float64 `json:"latest"`
	Avg    float64 `json:"avg"`
	Tail   float64 `json:"tail"`
	Loss   float64 `json:"loss"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

type NodeStatus struct {
	Client         string                `json:"client"`
	Time           string                `json:"time"`
	CPU            float64               `json:"cpu"`
	GPU            float64               `json:"gpu"`
	RAM            int64                 `json:"ram"`
	RAMTotal       int64                 `json:"ram_total"`
	Swap           int64                 `json:"swap"`
	SwapTotal      int64                 `json:"swap_total"`
	Load           float64               `json:"load"`
	Load5          float64               `json:"load5"`
	Load15         float64               `json:"load15"`
	Temp           float64               `json:"temp"`
	Disk           int64                 `json:"disk"`
	DiskTotal      int64                 `json:"disk_total"`
	NetIn          int64                 `json:"net_in"`
	NetOut         int64                 `json:"net_out"`
	NetTotalUp     int64                 `json:"net_total_up"`
	NetTotalDown   int64                 `json:"net_total_down"`
	Process        int64                 `json:"process"`
	Connections    int64                 `json:"connections"`
	ConnectionsUDP int64                 `json:"connections_udp"`
	Online         bool                  `json:"online"`
	Uptime         int64                 `json:"uptime"`
	Ping           map[string]PingResult `json:"ping"`
}

// ──────────────────────────────────────────────────────────────────
// common:getRecords
// ──────────────────────────────────────────────────────────────────

type RecordsResponse struct {
	Count     int    `json:"count"`
	From      string `json:"from"`
	To        string `json:"to"`
	Records   any    `json:"records"`
	LoadType  string `json:"load_type,omitempty"`  // 仅 load+指定 load_type 时
	BasicInfo any    `json:"basic_info,omitempty"` // 仅 ping 类型
	Tasks     any    `json:"tasks,omitempty"`      // 仅 ping 类型
}

// ──────────────────────────────────────────────────────────────────
// common:getNodeRecentStatus
// ──────────────────────────────────────────────────────────────────

type RecentStatusRecord struct {
	Client         string   `json:"client"`
	Time           string   `json:"time"`
	CPU            *float64 `json:"cpu,omitempty"`
	GPU            *float64 `json:"gpu,omitempty"`
	RAM            *int64   `json:"ram,omitempty"`
	RAMTotal       *int64   `json:"ram_total,omitempty"`
	Swap           *int64   `json:"swap,omitempty"`
	SwapTotal      *int64   `json:"swap_total,omitempty"`
	Load           *float64 `json:"load,omitempty"`
	Temp           *float64 `json:"temp,omitempty"`
	Disk           *int64   `json:"disk,omitempty"`
	DiskTotal      *int64   `json:"disk_total,omitempty"`
	NetIn          *int64   `json:"net_in,omitempty"`
	NetOut         *int64   `json:"net_out,omitempty"`
	NetTotalUp     *int64   `json:"net_total_up,omitempty"`
	NetTotalDown   *int64   `json:"net_total_down,omitempty"`
	Process        *int64   `json:"process,omitempty"`
	Connections    *int64   `json:"connections,omitempty"`
	ConnectionsUDP *int64   `json:"connections_udp,omitempty"`
}

type RecentStatusResp struct {
	Count   int                  `json:"count"`
	Records []RecentStatusRecord `json:"records"`
}

// ──────────────────────────────────────────────────────────────────
// common:getPublicInfo
// ──────────────────────────────────────────────────────────────────

type PublicInfo struct {
	AllowCORs              bool           `json:"allow_cors"`
	CustomBody             string         `json:"custom_body"`
	CustomHead             string         `json:"custom_head"`
	Description            string         `json:"description"`
	DisablePasswordLogin   bool           `json:"disable_password_login"`
	OauthEnable            bool           `json:"oauth_enable"`
	OauthProvider          string         `json:"oauth_provider"`
	PingRecordPreserveTime int            `json:"ping_record_preserve_time"`
	PrivateSite            bool           `json:"private_site"`
	RecordEnabled          bool           `json:"record_enabled"`
	RecordPreserveTime     int            `json:"record_preserve_time"`
	SiteName               string         `json:"sitename"`
	Theme                  string         `json:"theme"`
	ThemeSettings          map[string]any `json:"theme_settings"`
}

// ──────────────────────────────────────────────────────────────────
// common:getVersion
// ──────────────────────────────────────────────────────────────────

type VersionInfo struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
}
