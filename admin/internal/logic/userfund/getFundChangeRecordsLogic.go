/*
 * 文件名称: getFundChangeRecordsLogic.go
 * 功能说明: 获取用户资金变动明细（充值和扣减记录）
 *
 * 主要功能:
 * - 查询指定用户的充值和扣减记录
 * - 支持筛选：全部 / 仅充值 / 仅扣减
 * - 自动根据 charge_source 映射充值方式说明
 * - 按时间倒序排列，最新记录在前
 *
 * 数据来源:
 * - ae_user_recharge_record: 充值记录表
 *   字段: pay_time, xlcredit_amount, charge_source
 *
 * 充值方式映射:
 * - 1: 管理员后台充值
 * - 2: 支付宝充值
 * - 3: 微信充值
 * - 4: 银行卡充值
 * - 其他: 系统充值
 *
 * 金额说明:
 * - xlcredit_amount > 0: 充值（前端显示绿色 +）
 * - xlcredit_amount < 0: 扣减（前端显示红色 -）
 *
 * API路由: GET /manager/api/userfund/user/:user_id/fund-changes?filter=all
 */

package userfund

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFundChangeRecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetFundChangeRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFundChangeRecordsLogic {
	return &GetFundChangeRecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFundChangeRecordsLogic) GetFundChangeRecords(req *types.FundChangeRecordReq) (resp *types.FundChangeRecordResp, err error) {
	// 构建查询条件
	whereClause := "user_id = $1"
	args := []interface{}{req.UserId}

	// 根据filter过滤
	if req.Filter == "recharge" {
		whereClause += " AND xlcredit_amount > 0"
	} else if req.Filter == "deduction" {
		whereClause += " AND xlcredit_amount < 0"
	}
	// filter == "all" 或不传，则不过滤

	// 充值类型筛选
	if req.ChargeType > 0 {
		whereClause += fmt.Sprintf(" AND charge_type = %d", req.ChargeType)
	}

	// 查询充值记录表（包含充值和扣减）
	query := fmt.Sprintf(`
		SELECT 
			pay_time,
			xlcredit_amount,
			charge_source,
			charge_type,
			remark,
			operator
		FROM ae_user_recharge_record
		WHERE %s
		ORDER BY pay_time DESC
		LIMIT 100
	`, whereClause)

	type recordRow struct {
		PayTime        time.Time `db:"pay_time"`
		XlcreditAmount float64   `db:"xlcredit_amount"`
		ChargeSource   int64     `db:"charge_source"`
		ChargeType     int64     `db:"charge_type"`
		Remark         sql.NullString `db:"remark"`
		Operator       sql.NullString `db:"operator"`
	}

	var rows []recordRow
	err = l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows, query, args...)
	if err != nil {
		l.Logger.Errorf("Failed to query fund change records: %v", err)
		return &types.FundChangeRecordResp{List: []types.FundChangeRecordItem{}}, nil
	}

	var records []types.FundChangeRecordItem
	for _, row := range rows {
		// 根据金额和charge_source判断类型
		var typeDesc string
		if row.XlcreditAmount > 0 {
			// 充值类型
			switch row.ChargeSource {
			case 1:
				typeDesc = "管理员后台充值"
			case 2:
				typeDesc = "支付宝充值"
			case 3:
				typeDesc = "微信充值"
			case 4:
				typeDesc = "银行卡充值"
			default:
				typeDesc = "系统充值"
			}
		} else {
			// 扣减类型：根据 charge_source 判断
			if row.ChargeSource == -1 {
				typeDesc = "系统扣减"
			} else if row.ChargeSource == 1 {
				typeDesc = "管理员后台扣减"
			} else {
				typeDesc = "系统扣减"
			}
		}

		// 类型说明（充值/扣减都展示选择的类型）
		var chargeTypeDesc string
		switch row.ChargeType {
		case 1:
			chargeTypeDesc = "用户常规充值"
		case 2:
			chargeTypeDesc = "系统故障补偿"
		case 3:
			chargeTypeDesc = "活动赠送"
		default:
			chargeTypeDesc = "未知"
		}

		operatorName := row.Operator.String
		if operatorName == "" {
			operatorName = "unknown"
		}

		records = append(records, types.FundChangeRecordItem{
			TransactionTime: row.PayTime.Format("2006-01-02 15:04:05"),
			TypeDescription: typeDesc,
			ChangeAmount:    row.XlcreditAmount,
			Status:          "成功",
			Remarks:         row.Remark.String,
			ChargeType:      row.ChargeType,
			ChargeTypeDesc:  chargeTypeDesc,
			Operator:        operatorName,
		})
	}

	return &types.FundChangeRecordResp{
		List: records,
	}, nil
}
