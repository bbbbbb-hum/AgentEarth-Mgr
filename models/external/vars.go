package external

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var ErrNotFound = sqlx.ErrNotFound

const (
	CMD_EXT_PATH = "路径替换"
	URLKEY       = "地址栏密钥"
	URLVALUE     = "地址栏密钥值"
	HEARDERKEY   = "请求头密钥"
	HEARDERVALUE = "请求头密钥值"
	ENVKEY       = "环境密钥"
	ENVVALUE     = "环境密钥值"
)
