package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	DB   DBConfig
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	McpTest McpTestConfig
	K8sSync K8sSyncConfig
}

// K8sSyncConfig K8s集群状态同步配置
type K8sSyncConfig struct {
	// Enabled 是否启用 K8s 同步（非集群环境设为 false）
	Enabled bool `json:",default=true"`
	// Namespace 要检测的命名空间，留空则从 ServiceAccount 自动读取
	Namespace string `json:",optional"`
}

type DBConfig struct {
	DataSource string
}

// McpTestConfig MCP测试配置
type McpTestConfig struct {
	// ServiceUrlPattern wemcp服务地址模式, {name}会被替换为wemcp_name
	// 示例: "http://ae-{name}-service:8081"
	ServiceUrlPattern string `json:",default=http://ae-{name}-service:8081"`
	// DefaultTimeout 默认超时时间(秒)
	DefaultTimeout int `json:",default=30"`
}
