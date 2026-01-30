// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"context"
	"database/sql"
	"errors"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	configModel "AgentEarth-Mgr/models/config"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateServiceConfigManualLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateServiceConfigManualLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateServiceConfigManualLogic {
	return &CreateServiceConfigManualLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateServiceConfigManualLogic) CreateServiceConfigManual(req *types.CreateServiceConfigManualReq) (resp *types.BaseResp, err error) {
	serviceConfig := &configModel.AeMcpExternalServicesConfig{
		Name:            req.Name,
		Type:            req.Type,
		Description:     req.Description,
		ProjectName:     req.ProjectName,
		MaxInstance:     req.MaxInstance,
		LaunchInfo:      req.LaunchInfo,
		ConnectInfo:     req.ConnectInfo,
		InstallInfo:     sql.NullString{String: req.InstallInfo, Valid: req.InstallInfo != ""},
		AccountRequired: sql.NullInt64{Int64: req.AccountRequired, Valid: true},
		TestStatus:      sql.NullInt64{Int64: req.TestStatus, Valid: true},
		OnlineStatus:    sql.NullInt64{Int64: req.OnlineStatus, Valid: true},
	}

	result, err := l.svcCtx.TaskNodeConfigModel.Insert(l.ctx, serviceConfig)
	if err != nil && isExternalServiceConfigPkConflict(err) {
		if resetErr := l.resetExternalServiceConfigSeq(); resetErr == nil {
			result, err = l.svcCtx.TaskNodeConfigModel.Insert(l.ctx, serviceConfig)
		}
	}
	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "创建服务配置失败: " + err.Error(),
		}, nil
	}

	serviceId, _ := result.LastInsertId()

	return &types.BaseResp{
		Code:    0,
		Message: "创建服务配置成功",
		Data: types.D{
			"service_id": serviceId,
		},
	}, nil
}

func isExternalServiceConfigPkConflict(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505" && pqErr.Constraint == "ae_mcp_remote_services_pkey"
	}
	return false
}

func (l *CreateServiceConfigManualLogic) resetExternalServiceConfigSeq() error {
	query := `select setval('"public"."ae_mcp_external_services_config_id_seq"', (select coalesce(max(id), 0) + 1 from "public"."ae_mcp_external_services_config"), false)`
	_, err := l.svcCtx.DB.ExecCtx(l.ctx, query)
	return err
}

// 移除不再需要的 generateServerId 函数，因为 ServerId 生成已移至 createServiceLogic.go
