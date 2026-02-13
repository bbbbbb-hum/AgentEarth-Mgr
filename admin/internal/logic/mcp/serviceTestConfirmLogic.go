package mcp

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceTestConfirmLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceTestConfirmLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceTestConfirmLogic {
	return &ServiceTestConfirmLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceTestConfirmLogic) ServiceTestConfirm(req *types.ServiceTestConfirmReq) (resp *types.BaseResp, err error) {
	// 1. 根据ConfigId查询服务配置
	config, err := l.svcCtx.TaskNodeConfigV2Model.FindOne(l.ctx, req.ConfigId)
	if err != nil {
		l.Logger.Errorf("查询服务配置失败: %v", err)
		return &types.BaseResp{
			Code:    1,
			Message: "服务配置不存在",
			Data:    types.D{},
		}, nil
	}

	// 2. 验证TestStatus值
	if req.TestStatus != 1 && req.TestStatus != -1 && req.TestStatus != 0 {
		return &types.BaseResp{
			Code:    1,
			Message: "无效的测试状态值，必须是 1(通过)、-1(失败) 或 0(未测试)",
			Data:    types.D{},
		}, nil
	}

	// 3. 更新测试状态
	config.TestStatus = int64(req.TestStatus)
	err = l.svcCtx.TaskNodeConfigV2Model.Update(l.ctx, config)
	if err != nil {
		l.Logger.Errorf("更新测试状态失败: %v", err)
		return &types.BaseResp{
			Code:    1,
			Message: "更新测试状态失败: " + err.Error(),
			Data:    types.D{},
		}, nil
	}

	// 4. 返回成功
	statusText := "未测试"
	switch req.TestStatus {
	case 1:
		statusText = "测试通过"
	case -1:
		statusText = "测试失败"
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"config_id":   config.Id,
			"wemcp_name":  config.WemcpName,
			"test_status": req.TestStatus,
			"status_text": statusText,
		},
	}, nil
}
