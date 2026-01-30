/*
 * 文件名称: getBalanceHistoryLogic.go
 * 功能说明: 获取用户余额历史（用于余额趋势曲线图）
 *
 * 主要功能:
 * - 查询指定用户最近7天或30天的余额历史
 * - 按日期升序排序
 * - 前端接收后渲染为面积曲线图，并自动滑动到最新一天
 *
 * 数据来源:
 * - ae_user_balance_statistic_daily: 日余额统计表（字段: day, balance）
 *
 * API路由: GET /manager/api/userfund/user/:user_id/balance?days=7
 *
 * 注意事项:
 * - 返回日期格式为 YYYY-MM-DD
 * - 余额为 numeric(16,6) 类型，映射为 float64 返回
 */

package userfund

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBalanceHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetBalanceHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBalanceHistoryLogic {
	return &GetBalanceHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetBalanceHistoryLogic) GetBalanceHistory(req *types.BalanceHistoryReq) (resp *types.BalanceHistoryResp, err error) {
	// 确保天数为7或30
	days := req.Days
	if days != 7 && days != 30 {
		days = 7
	}

	// 查询余额历史（从余额统计表）- 包含今天，查询最近N天的数据
	// 对缺失日期用“最新可用余额”进行衔接
	query := `
		WITH date_series AS (
			SELECT generate_series(
				CURRENT_DATE - INTERVAL '1 day' * ($2 - 1),
				CURRENT_DATE,
				INTERVAL '1 day'
			)::date AS day
		)
		SELECT 
			ds.day::text as day,
			COALESCE((
				SELECT s.balance
				FROM ae_user_balance_statistic_daily s
				WHERE s.user_id = $1
				AND s.day <= ds.day
				ORDER BY s.day DESC
				LIMIT 1
			), 0) as balance
		FROM date_series ds
		ORDER BY ds.day ASC
	`

	type balanceRow struct {
		Day     string  `db:"day"`
		Balance float64 `db:"balance"`
	}

	var rows []balanceRow
	err = l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows, query, req.UserId, days)
	if err != nil {
		l.Logger.Errorf("Failed to query balance history: %v", err)
		return &types.BalanceHistoryResp{List: []types.BalanceHistoryItem{}}, nil
	}

	var records []types.BalanceHistoryItem
	for _, row := range rows {
		records = append(records, types.BalanceHistoryItem{
			Day:     row.Day,
			Balance: row.Balance,
		})
	}

	return &types.BalanceHistoryResp{
		List: records,
	}, nil
}
