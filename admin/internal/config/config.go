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
