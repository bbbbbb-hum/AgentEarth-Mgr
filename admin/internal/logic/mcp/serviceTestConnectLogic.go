package mcp

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/pkg/mcpclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceTestConnectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceTestConnectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceTestConnectLogic {
	return &ServiceTestConnectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceTestConnectLogic) ServiceTestConnect(req *types.ServiceTestConnectReq) (resp *types.BaseResp, err error) {
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

	// 2. 构建服务URL
	serviceURL := mcpclient.BuildServiceURL(
		l.svcCtx.Config.McpTest.ServiceUrlPattern,
		config.WemcpName,
	)

	// 3. 设置超时时间
	timeout := l.svcCtx.Config.McpTest.DefaultTimeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	// 4. 创建MCP客户端并连接
	client := mcpclient.NewClient(serviceURL, time.Duration(timeout)*time.Second)
	result, err := client.Connect(l.ctx)
	if err != nil {
		l.Logger.Errorf("MCP连接失败: %v", err)
		return &types.BaseResp{
			Code:    1,
			Message: "连接失败",
			Data: types.D{
				"error": err.Error(),
			},
		}, nil
	}

	// 5. 返回结果
	if !result.Success {
		return &types.BaseResp{
			Code:    1,
			Message: "连接测试失败",
			Data: types.D{
				"success":     false,
				"error":       result.Error,
				"duration_ms": result.DurationMs,
			},
		}, nil
	}

	// 转换工具列表为可序列化格式
	toolList := make([]map[string]interface{}, 0, len(result.Tools))
	for _, tool := range result.Tools {
		toolItem := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
		}
		if tool.InputSchema != nil {
			toolItem["inputSchema"] = tool.InputSchema
		}
		toolList = append(toolList, toolItem)
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"success":     true,
			"config_id":   config.Id,
			"wemcp_name":  config.WemcpName,
			"service_url": serviceURL,
			"server_info": result.ServerInfo,
			"tools":       toolList,
			"tools_count": result.ToolsCount,
			"duration_ms": result.DurationMs,
		},
	}, nil
}
