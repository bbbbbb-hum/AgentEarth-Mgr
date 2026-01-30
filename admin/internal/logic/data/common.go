package data

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func CreateProjectName(name string) string {
	// 生成项目名 用 @符号+name 去拼接 把name中的空格去掉，然后空格后的首字母改大写
	projectName := strings.ReplaceAll(name, " ", "")
	projectName = strings.Title(projectName)
	return "@" + projectName
}

func FormatServerId(id int64) string {
	return fmt.Sprintf("server_%07d", id)
}

func NextServiceId(ctx context.Context, conn sqlx.SqlConn) (int64, error) {
	query := `select nextval('"public"."ae_mcp_server_id_seq"')`
	var id int64
	if err := conn.QueryRowCtx(ctx, &id, query); err != nil {
		return 0, err
	}
	return id, nil
}
