package rules

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ExecutionDetailsLogic 对账-执行明细：某次批次下的每条用户执行记录
type ExecutionDetailsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExecutionDetailsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecutionDetailsLogic {
	return &ExecutionDetailsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExecutionDetailsLogic) GetRuleExecutionDetails(req *types.RuleExecutionDetailsReq) (resp *types.RuleExecutionDetailsResp, err error) {
	chargeSource := req.ChargeSource
	// 兜底：前端没传时默认为 4（自动 Rule 操作），但正常应始终由前端显式传入 3/4。
	if chargeSource == 0 {
		chargeSource = 4
	}

	total, rows, err := l.svcCtx.RuleQueryModel.GetExecutionDetails(l.ctx, req.RuleId, req.RunTime, chargeSource)
	if err != nil {
		return nil, err
	}

	items := make([]types.RuleExecutionDetailItem, 0, len(rows))
	for _, r := range rows {
		userName := ""
		if r.UserName.Valid {
			userName = r.UserName.String
		}
		operatorName := ""
		if r.OperatorName.Valid {
			operatorName = r.OperatorName.String
		}
		items = append(items, types.RuleExecutionDetailItem{
			TriggerTime:  r.TriggerTime,
			ExecSource:   r.ExecSource,
			UserId:       r.UserId,
			UserName:     userName,
			Operator:     operatorName,
			ChangeAmount: r.ChangeAmount,
			Status:       r.Status,
			ActionType:   r.ActionType,
		})
	}

	return &types.RuleExecutionDetailsResp{
		Total: total,
		List:  items,
	}, nil
}
