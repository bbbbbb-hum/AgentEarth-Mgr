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
	// 临时方案：不使用 setval (避免 UPDATE 权限要求)，仅通过 nextval 实现
	// 1. 获取当前序列值和表中的最大 ID
	// 2. 通过 generate_series 批量调用 nextval 来“跳过”已存在的 ID 并实现随机递增
	// 这样即便序列号(Sequence)落后于实际数据，也能自动校准
	query := `
		SELECT nextval('"public"."ae_mcp_server_id_seq"')
		FROM (
			SELECT 
				nextval('"public"."ae_mcp_server_id_seq"') as seq_val,
				(SELECT coalesce(max(id), 0) FROM "public"."ae_mcp_services") as max_id
		) s, generate_series(1, GREATEST(1, (max_id - seq_val + 1 + floor(random() * 9))::int))
		ORDER BY 1 DESC
		LIMIT 1`
	var id int64
	if err := conn.QueryRowCtx(ctx, &id, query); err != nil {
		return 0, err
	}
	return id, nil
}
