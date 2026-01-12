package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	DB     DBConfig
	ProdDB DBConfig
	Auth   struct {
		AccessSecret string
		AccessExpire int64
	}
}
type DBConfig struct {
	DataSource string
}
