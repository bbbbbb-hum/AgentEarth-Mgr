package mcp

import (
	"context"
	"encoding/json"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/pkg/mcpclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceTestCallLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceTestCallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceTestCallLogic {
	return &ServiceTestCallLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceTestCallLogic) ServiceTestCall(req *types.ServiceTestCallReq) (resp *types.BaseResp, err error) {
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

	// 3. 解析参数
	var arguments map[string]interface{}
	if req.Arguments != "" {
		if err := json.Unmarshal([]byte(req.Arguments), &arguments); err != nil {
			return &types.BaseResp{
				Code:    1,
				Message: "参数格式错误: " + err.Error(),
				Data:    types.D{},
			}, nil
		}
	} else {
		arguments = make(map[string]interface{})
	}

	// 4. 设置超时时间
	timeout := l.svcCtx.Config.McpTest.DefaultTimeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	// 5. 使用会话管理器获取或创建客户端连接
	sessionMgr := mcpclient.GetSessionManager()
	session, cached, err := sessionMgr.GetOrCreate(
		l.ctx,
		req.ConfigId,
		config.WemcpName,
		serviceURL,
		time.Duration(timeout)*time.Second,
	)
	if err != nil {
		l.Logger.Errorf("MCP连接初始化失败: %v", err)
		return &types.BaseResp{
			Code:    1,
			Message: "连接初始化失败: " + err.Error(),
			Data:    types.D{},
		}, nil
	}

	if cached {
		l.Logger.Infof("使用缓存的MCP会话: configId=%d", req.ConfigId)
	} else {
		l.Logger.Infof("创建新的MCP会话: configId=%d", req.ConfigId)
	}

	// 6. 调用工具
	result, err := session.Client.CallTool(l.ctx, req.ToolName, arguments)
	if err != nil {
		// 如果调用失败，可能是会话已过期，移除缓存
		sessionMgr.Remove(req.ConfigId)
		l.Logger.Errorf("工具调用失败: %v", err)
		return &types.BaseResp{
			Code:    1,
			Message: "工具调用失败",
			Data: types.D{
				"error": err.Error(),
			},
		}, nil
	}

	// 更新最后使用时间
	sessionMgr.UpdateLastUsed(req.ConfigId)

	// 7. 返回结果
	if !result.Success {
		return &types.BaseResp{
			Code:    1,
			Message: "工具调用失败",
			Data: types.D{
				"success":      false,
				"error":        result.Error,
				"duration_ms":  result.DurationMs,
				"session_hit":  cached,
			},
		}, nil
	}

	// 解析content为JSON对象
	var content interface{}
	if result.Content != nil {
		if err := json.Unmarshal(result.Content, &content); err != nil {
			content = string(result.Content)
		}
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"success":      true,
			"tool_name":    req.ToolName,
			"content":      content,
			"is_error":     result.IsError,
			"duration_ms":  result.DurationMs,
			"session_hit":  cached,
		},
	}, nil
}
