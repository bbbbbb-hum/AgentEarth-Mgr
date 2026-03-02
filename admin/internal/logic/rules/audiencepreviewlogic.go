package rules

import (
	"context"
	"fmt"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	ruleModel "AgentEarth-Mgr/models/rules"

	"github.com/zeromicro/go-zero/core/logx"
)

// AudiencePreviewLogic 受众预览：按与规则执行相同的筛选条件查询命中用户，分页返回全部
type AudiencePreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAudiencePreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AudiencePreviewLogic {
	return &AudiencePreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AudiencePreview 返回当前筛选条件将命中的用户总数和当前页列表（与 ExecuteRule 筛选逻辑一致）
func (l *AudiencePreviewLogic) AudiencePreview(req *types.AudiencePreviewReq) (*types.AudiencePreviewResp, error) {
	page := req.Page
	size := req.Size
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 2000 {
		size = 500
	}
	offset := (page - 1) * size

	filter := ruleModel.AudienceFilter{
		MinRegDays:          int(req.MinRegDays),
		MaxRegDays:          req.MaxRegDays,
		LastLoginWithinDays: req.LastLoginWithinDays,
		MinLastMonthConsume: req.MinLastMonthConsume,
		MaxLastMonthConsume: req.MaxLastMonthConsume,
		MinBalance:          req.MinBalance,
		MaxBalance:          req.MaxBalance,
		RegChannel:          req.RegChannel,
		MinHistoryRecharge:  req.MinHistoryRecharge,
		MaxHistoryRecharge:  req.MaxHistoryRecharge,
	}

	total, err := l.svcCtx.RuleQueryModel.CountAudience(l.ctx, filter)
	if err != nil {
		l.Logger.Errorf("受众预览统计总数失败: %v", err)
		return nil, fmt.Errorf("查询命中用户总数失败: %w", err)
	}

	rows, err := l.svcCtx.RuleQueryModel.ListAudience(l.ctx, filter, size, offset)
	if err != nil {
		l.Logger.Errorf("受众预览查询列表失败: %v", err)
		return nil, fmt.Errorf("查询命中用户列表失败: %w", err)
	}

	list := make([]types.AudiencePreviewUserItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, types.AudiencePreviewUserItem{
			UserId:   r.UserId,
			Username: r.Username,
		})
	}

	return &types.AudiencePreviewResp{
		Total: total,
		List:  list,
	}, nil
}
