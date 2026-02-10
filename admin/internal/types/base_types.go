package types

type (
	D map[string]interface{}

	FlatLaunch struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
		Workdir string            `json:"workdir"`
	}
	NestedLaunch struct {
		McpServers map[string]FlatLaunch `json:"mcpServers"`
	}
	TargetLaunch struct {
		Command         string            `json:"command"`
		Args            []string          `json:"args"`
		Workdir         string            `json:"workdir,omitempty"`
		Env             map[string]string `json:"env,omitempty"`
		MaxInstance     int               `json:"max_instance"`
		MaxRestarts     int               `json:"max_restarts"`
		LaunchTimeout   int               `json:"launch_timeout"`
		ShutdownTimeout int               `json:"shutdown_timeout"`
		IdleTtl         int               `json:"idle_ttl"`
	}

	FlatConnect struct {
		URL     string            `json:"url"`
		Type    string            `json:"type,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
	}
	NestedConnect struct {
		McpServers map[string]FlatConnect `json:"mcpServers"`
	}
	TargetConnect struct {
		URL            string            `json:"url"`
		Headers        map[string]string `json:"headers,omitempty"`
		ConnectTimeout int               `json:"connect_timeout"`
		MaxConnect     int               `json:"max_connect"`
		MaxRetry       int               `json:"max_retry"`
		Interval       int               `json:"interval"`
	}

	// ==================== MCP服务测试类型 ====================
	// ServiceTestConnectReq 连接测试请求
	ServiceTestConnectReq struct {
		ConfigId int64 `json:"config_id"`        // 服务配置ID (ae_mcp_external_services_config_v2.id)
		Timeout  int   `json:"timeout,optional"` // 超时秒数，默认30
	}

	// ServiceTestCallReq 工具调用请求
	ServiceTestCallReq struct {
		ConfigId  int64  `json:"config_id"`          // 服务配置ID
		ToolName  string `json:"tool_name"`          // 工具名称
		Arguments string `json:"arguments,optional"` // 工具参数JSON字符串
		Timeout   int    `json:"timeout,optional"`   // 超时秒数，默认30
	}

	// ServiceTestConfirmReq 确认测试结果请求
	ServiceTestConfirmReq struct {
		ConfigId   int64 `json:"config_id"`   // 服务配置ID
		TestStatus int   `json:"test_status"` // 测试状态: 1=通过, -1=失败, 0=未测试
	}

	// ServiceTestDisconnectReq 断开连接请求
	ServiceTestDisconnectReq struct {
		ConfigId int64 `json:"config_id"` // 服务配置ID
	}
)
