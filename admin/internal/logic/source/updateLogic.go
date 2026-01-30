package source

import (
	"context"
	"database/sql"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLogic {
	return &UpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateLogic) Update(req *types.SourceUpdateReq) (resp *types.BaseResp, err error) {
	detail, err := l.svcCtx.ExternalMcpServicesModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return
	}
	if len(req.ServerName) > 0 {
		detail.ServerName = req.ServerName
	}
	if len(req.ServerType) > 0 {
		detail.ServerType = req.ServerType
	}
	if len(req.LaunchInfo) > 0 {
		detail.LaunchInfo = req.LaunchInfo
	}
	if len(req.ConnectInfo) > 0 {
		detail.ConnectInfo = req.ConnectInfo
	}
	if len(req.Description) > 0 {
		detail.Description = req.Description
	}
	if len(req.CodeSourceUrl) > 0 {
		detail.CodeSourceUrl = req.CodeSourceUrl
	}
	if req.TestStatus > 0 {
		detail.TestStatus = req.TestStatus
	}
	if len(req.DockerCmd) > 0 {
		detail.DockerCmd = sql.NullString{
			String: req.DockerCmd,
			Valid:  true,
		}
	}
	if len(req.ProjectName) > 0 {
		detail.ProjectName = sql.NullString{
			String: req.ProjectName,
			Valid:  true,
		}
	}
	if len(req.Valuable) > 0 {
		detail.Valuable = sql.NullString{
			String: req.Valuable,
			Valid:  true,
		}
	}
	if req.Calls > 0 {
		detail.Calls = sql.NullInt64{
			Int64: req.Calls,
			Valid: true,
		}
	}
	if len(req.Category) > 0 {
		detail.Category = sql.NullString{
			String: req.Category,
			Valid:  true,
		}
	}
	if len(req.Image) > 0 {
		detail.Image = sql.NullString{
			String: req.Image,
			Valid:  true,
		}
	}
	if len(req.CloneRepository) > 0 {
		detail.CloneRepository = sql.NullString{
			String: req.CloneRepository,
			Valid:  true,
		}
	}
	if req.NeedKey > 0 {
		detail.NeedKey = sql.NullInt64{
			Int64: req.NeedKey,
			Valid: true,
		}
	}
	if len(req.Install) > 0 {
		detail.Install = sql.NullString{
			String: req.Install,
			Valid:  true,
		}
	}
	detail.UpdateTime = sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}
	err = l.svcCtx.ExternalMcpServicesModel.Update(l.ctx, detail)
	if err != nil {
		return
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}
	return
}
