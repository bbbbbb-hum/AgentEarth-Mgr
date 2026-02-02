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
	// 核心逻辑：确保生成的 ID 永远大于当前数据库中的最大 ID
	// 1. 使用 GREATEST 函数对比 (序列下一个值) 和 (当前表最大 ID + 1)
	// 2. 在较大的那个值基础上，再增加 0-9 的随机数
	// 这样即便序列号(Sequence)因为数据库迁移或手动修改落后于实际数据，也能自动校准并跳过已存在的 ID
	query := `
		SELECT setval('"public"."ae_mcp_server_id_seq"', 
			GREATEST(
				nextval('"public"."ae_mcp_server_id_seq"'), 
				(SELECT coalesce(max(id), 0) + 1 FROM "public"."ae_mcp_services")
			) + floor(random() * 9)::bigint
		)`
	var id int64
	if err := conn.QueryRowCtx(ctx, &id, query); err != nil {
		return 0, err
	}
	return id, nil
}
