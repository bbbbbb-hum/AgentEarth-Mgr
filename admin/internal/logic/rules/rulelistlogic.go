// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2
// 规则列表：分页查询策略规则

package rules

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RuleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRuleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RuleListLogic {
	return &RuleListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RuleListLogic) GetRuleList(req *types.BaseListReq) (resp *types.RuleListResp, err error) {
	page := req.Page
	size := req.Size
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 10
	}
	offset := (page - 1) * size

	total, rows, err := l.svcCtx.RuleQueryModel.GetRuleList(l.ctx, req.Search, size, offset)
	if err != nil {
		return nil, err
	}

	items := make([]types.RuleItem, 0, len(rows))
	for _, r := range rows {
		lastExec := ""
		if !r.LastExecTime.IsZero() && r.LastExecTime.Year() > 1 {
			lastExec = r.LastExecTime.Format("2006-01-02 15:04:05")
		}

		items = append(items, types.RuleItem{
			Id:             r.Id,
			Name:           r.Name,
			Description:    r.Description,
			Priority:       r.Priority,
			IsActive:       r.Status == "active",
			CronExpression: r.TriggerKey,
			FilterConfig:   r.FilterConfig,
			ActionConfig:   r.ActionConfig,
			LastExecTime:   lastExec,
			CreateTime:     r.CreateTime.Format("2006-01-02 15:04:05"),
		})
	}

	return &types.RuleListResp{
		Total: total,
		List:  items,
	}, nil
}
