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
	start := time.Now()

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

	// 4. 使用会话管理器获取或创建客户端连接
	sessionMgr := mcpclient.GetSessionManager()
	session, cached, err := sessionMgr.GetOrCreate(
		l.ctx,
		req.ConfigId,
		config.WemcpName,
		serviceURL,
		time.Duration(timeout)*time.Second,
	)
	if err != nil {
		l.Logger.Errorf("MCP连接失败: %v", err)
		return &types.BaseResp{
			Code:    1,
			Message: "连接失败",
			Data: types.D{
				"success": false,
				"error":   err.Error(),
			},
		}, nil
	}

	if cached {
		l.Logger.Infof("使用缓存的MCP会话: configId=%d", req.ConfigId)
	} else {
		l.Logger.Infof("创建新的MCP会话: configId=%d", req.ConfigId)
	}

	// 5. 获取工具列表
	tools, err := session.Client.ListTools(l.ctx)
	if err != nil {
		// 如果获取失败，可能是会话已过期，移除缓存并重试
		sessionMgr.Remove(req.ConfigId)
		l.Logger.Errorf("获取工具列表失败: %v", err)
		return &types.BaseResp{
			Code:    1,
			Message: "获取工具列表失败",
			Data: types.D{
				"success": false,
				"error":   err.Error(),
			},
		}, nil
	}

	durationMs := time.Since(start).Milliseconds()

	// 转换工具列表为可序列化格式
	toolList := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
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
			"server_info": session.ServerInfo,
			"tools":       toolList,
			"tools_count": len(tools),
			"duration_ms": durationMs,
			"session_hit": cached,
		},
	}, nil
}
