// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"context"
	"errors"
	"regexp"
	"strings"

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
	wemcpName := strings.TrimSpace(req.WemcpName)
	if len(wemcpName) == 0 {
		return &types.BaseResp{
			Code:    -1,
			Message: "wemcp_name不能为空",
		}, nil
	}
	if !isValidWemcpName(wemcpName) {
		return &types.BaseResp{
			Code:    -1,
			Message: "wemcp_name格式不正确，应类似 wemcp2-qweather（小写字母/数字/连字符）",
		}, nil
	}

	serviceConfig := &configModel.AeMcpExternalServicesConfigV2{
		Name:            req.Name,
		Description:     req.Description,
		Comments:        req.Comments,
		CodeSourceUrl:   req.CodeSourceUrl,
		Tags:            pq.StringArray(req.Tags),
		AccountRequired: req.AccountRequired,
		TestStatus:      req.TestStatus,
		OnlineStatus:    req.OnlineStatus,
		WemcpName:       wemcpName,
	}

	result, err := l.svcCtx.TaskNodeConfigV2Model.Insert(l.ctx, serviceConfig)
	if err != nil && isExternalServiceConfigV2PkConflict(err) {
		if resetErr := l.resetExternalServiceConfigSeq(); resetErr == nil {
			result, err = l.svcCtx.TaskNodeConfigV2Model.Insert(l.ctx, serviceConfig)
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

func isExternalServiceConfigV2PkConflict(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505" && pqErr.Constraint == "ae_mcp_external_services_config_v2_pkey"
	}
	return false
}

func isValidWemcpName(name string) bool {
	re := regexp.MustCompile(`^wemcp2-[a-z0-9]+(?:-[a-z0-9]+)*$`)
	return re.MatchString(name)
}

func (l *CreateServiceConfigManualLogic) resetExternalServiceConfigSeq() error {
	query := `select setval('"public"."ae_mcp_external_services_config_v2_id_seq"', (select coalesce(max(id), 0) + 1 from "public"."ae_mcp_external_services_config_v2"), false)`
	_, err := l.svcCtx.DB.ExecCtx(l.ctx, query)
	return err
}
