package rules

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ExecutionRunsLogic 对账-执行批次列表：按规则查询每次执行的批次（时间+来源+成功/失败数）
type ExecutionRunsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExecutionRunsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecutionRunsLogic {
	return &ExecutionRunsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExecutionRunsLogic) GetRuleExecutionRuns(req *types.RuleExecutionRunsReq) (resp *types.RuleExecutionRunsResp, err error) {
	page := req.Page
	size := req.Size
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	total, rows, err := l.svcCtx.RuleQueryModel.GetExecutionRuns(l.ctx, req.RuleId, size, offset)
	if err != nil {
		return nil, err
	}

	items := make([]types.RuleExecutionRunItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, types.RuleExecutionRunItem{
			RunTime:      r.RunTime,
			ChargeSource: r.ChargeSource,
			ExecSource:   r.ExecSource,
			TotalCount:   r.TotalCount,
			SuccessCount: r.SuccessCount,
			FailedCount:  r.FailedCount,
		})
	}

	return &types.RuleExecutionRunsResp{
		Total: total,
		List:  items,
	}, nil
}
