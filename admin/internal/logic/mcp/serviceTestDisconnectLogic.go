package mcp

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/pkg/mcpclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceTestDisconnectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceTestDisconnectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceTestDisconnectLogic {
	return &ServiceTestDisconnectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceTestDisconnectLogic) ServiceTestDisconnect(req *types.ServiceTestDisconnectReq) (resp *types.BaseResp, err error) {
	sessionMgr := mcpclient.GetSessionManager()
	sessionMgr.Remove(req.ConfigId)

	l.Logger.Infof("MCP会话已释放: configId=%d", req.ConfigId)

	return &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}, nil
}
