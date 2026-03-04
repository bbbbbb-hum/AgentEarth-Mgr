package rules

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteRuleLogic {
	return &DeleteRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteRuleLogic) DeleteRule(req *types.DeleteRuleReq) (resp *types.BaseResp, err error) {
	l.Logger.Infof("尝试删除规则 ID: %d", req.Id)

	// 先从调度器中移除这条规则（如果调度器当前在本实例中运行的话）
	unregisterRuleFromScheduler(l.ctx, req.Id)

	// 仅删除规则本身（返回影响行数用于判断是否存在）。
	// 充值记录作为资金台账保留，不做级联删除，可通过 rule_id 字段进行历史追溯。
	rows, err := l.svcCtx.RuleQueryModel.DeleteRuleById(l.ctx, req.Id)
	if err != nil {
		l.Logger.Errorf("删除规则 %d 失败: %v", req.Id, err)
		return nil, err
	}
	if rows == 0 {
		l.Logger.Infof("规则 %d 未找到或已被删除", req.Id)
	} else {
		l.Logger.Infof("成功删除规则 %d", req.Id)
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    nil,
	}, nil
}
