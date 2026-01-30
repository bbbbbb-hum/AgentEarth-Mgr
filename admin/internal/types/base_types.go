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
)
