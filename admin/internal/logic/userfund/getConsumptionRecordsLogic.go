/*
 * 文件名称: getConsumptionRecordsLogic.go
 * 功能说明: 获取用户消费记录（用于资金流向柱状图）
 *
 * 主要功能:
 * - 查询指定用户最近7天或30天的消费记录
 * - 返回所有消费数据（xlcredit_consume > 0 为自行消费，< 0 为系统扣减）
 * - 按日期升序排序，便于前端图表渲染
 *
 * 数据来源:
 * - ae_user_consumption_record_daily: 日消费记录（字段: day, xlcredit_consume）
 *
 * API路由: GET /manager/api/userfund/user/:user_id/consumption?days=7
 *
 * 注意事项:
 * - 前端堆叠柱状图同时显示自行消费（绿色）和系统扣减（红色）
 * - xlcredit_consume 为 int4 类型，映射为 float64 返回
 */

package userfund

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConsumptionRecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetConsumptionRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConsumptionRecordsLogic {
	return &GetConsumptionRecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetConsumptionRecordsLogic) GetConsumptionRecords(req *types.ConsumptionRecordReq) (resp *types.ConsumptionRecordResp, err error) {
	// 确保天数为7或30
	days := req.Days
	if days != 7 && days != 30 {
		days = 7
	}

	// 查询消费记录（自行消费 + 充值表负值扣减），SQL 在 model 层
	rows, err := l.svcCtx.UserConsumptionDailyModel.QueryConsumptionRecords(l.ctx, req.UserId, int64(days))
	if err != nil {
		l.Logger.Errorf("Failed to query consumption records: %v", err)
		return &types.ConsumptionRecordResp{List: []types.ConsumptionRecordItem{}}, nil
	}

	var records []types.ConsumptionRecordItem
	for _, row := range rows {
		records = append(records, types.ConsumptionRecordItem{
			Day:          row.Day,
			SelfConsume:  row.SelfConsume,
			SystemDeduct: row.SystemDeduct,
		})
	}

	return &types.ConsumptionRecordResp{
		List: records,
	}, nil
}
