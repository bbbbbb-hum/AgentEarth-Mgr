package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	DB     DBConfig
	ProdDB DBConfig
}
type DBConfig struct {
	DataSource string
}
