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
 * - ae_mgrsystem_user: 管理员表 (用于关联操作人名称)
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
	"context"      //上下文
	"database/sql" //sql.NullString、sql.NullTime等可空类型
	"fmt"          //格式化
	"time"         //时间

	"AgentEarth-Mgr/admin/internal/svc"   //服务上下文
	"AgentEarth-Mgr/admin/internal/types" //请求/响应类型

	"github.com/shopspring/decimal"          //高精度金额
	"github.com/zeromicro/go-zero/core/logx" //日志
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
	// 分页默认值
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 构建查询条件 (在sql语句中使用别名 r 指代 ae_user_recharge_record)
	whereClause := "r.user_id = $1"
	//定义一个切片，类型为interface{}空接口，可以是任何类型,<Object>
	//{req.UserId}相当于给切片一个初始化赋值
	args := []interface{}{req.UserId}
	argIdx := 2 //计数器，$1已经被user_id占用了，如果后面还有查询条件，那么条件的占位符就得叫$2

	// 根据filter过滤，用全部、仅充值、仅扣减筛选时的sql
	if req.Filter == "recharge" {
		whereClause += " AND r.xlcredit_amount > 0"
	} else if req.Filter == "deduction" {
		whereClause += " AND r.xlcredit_amount < 0"
	}

	// 充值类型筛选：参数化查询
	if req.ChargeType > 0 {
		whereClause += fmt.Sprintf(" AND r.charge_type = $%d", argIdx)
		args = append(args, req.ChargeType)
		argIdx++
	}

	// 先查总数
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM ae_user_recharge_record r
		LEFT JOIN ae_mgrsystem_user u ON u.user_id::text = r.operator
		WHERE %s
	`, whereClause)
	var total int64
	err = l.svcCtx.DB.QueryRowCtx(l.ctx, &total, countQuery, args...)
	if err != nil {
		l.Logger.Errorf("查询充值记录总数失败...: %v", err)
		return &types.FundChangeRecordResp{List: []types.FundChangeRecordItem{}, Total: 0}, nil
	}

	// 查询充值记录表（包含充值和扣减），并关联管理员表获取用户名
	// r.id、r.expire_time 用于充值时展示批次剩余额度和过期状态
	limitArgs := append(args, pageSize, offset)
	query := fmt.Sprintf(`
		SELECT 
			r.id,
			r.pay_time,
			r.xlcredit_amount,
			r.charge_source,
			r.charge_type,
			r.remark,
			r.expire_time,
			COALESCE(u.username, r.operator) as operator
		FROM ae_user_recharge_record r
		LEFT JOIN ae_mgrsystem_user u ON u.user_id::text = r.operator
		WHERE %s
		ORDER BY r.pay_time DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	type recordRow struct {
		Id             int64          `db:"id"`
		PayTime        time.Time      `db:"pay_time"`
		XlcreditAmount float64        `db:"xlcredit_amount"`
		ChargeSource   int64          `db:"charge_source"`
		ChargeType     int64          `db:"charge_type"`
		Remark         sql.NullString `db:"remark"`
		ExpireTime     sql.NullTime   `db:"expire_time"`
		Operator       sql.NullString `db:"operator"`
	}

	var rows []recordRow
	err = l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows, query, limitArgs...)
	if err != nil {
		l.Logger.Errorf("Failed to query fund change records: %v", err)
		return &types.FundChangeRecordResp{List: []types.FundChangeRecordItem{}, Total: 0}, nil
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
		case 4:
			chargeTypeDesc = "过期扣减"
		case 5:
			chargeTypeDesc = "管理员扣减"
		default:
			chargeTypeDesc = "未知"
		}

		//设置操作人用户名
		operatorName := row.Operator.String
		if operatorName == "" {
			operatorName = "未知"
		}

		item := types.FundChangeRecordItem{
			TransactionTime: row.PayTime.Format("2006-01-02 15:04:05"),
			TypeDescription: typeDesc,
			ChangeAmount:    row.XlcreditAmount,
			Status:          "成功",
			Remarks:         row.Remark.String,
			ChargeType:      row.ChargeType,
			ChargeTypeDesc:  chargeTypeDesc,
			Operator:        operatorName,
		}

		// 仅充值时计算批次剩余额度、过期时间、状态
		if row.XlcreditAmount > 0 {
			item.BatchId = row.Id
			item.InitialAmount = row.XlcreditAmount
			initialAmt := decimal.NewFromFloat(row.XlcreditAmount)
			remaining, errBalance := CalculateRealTimeBalance(l.ctx, l.svcCtx.DB, row.Id, initialAmt)
			if errBalance == nil {
				item.RemainingAmount, _ = remaining.Float64()
			}
			if row.ExpireTime.Valid {
				item.ExpireTime = row.ExpireTime.Time.Format("2006-01-02")
				now := time.Now()
				if row.ExpireTime.Time.Before(now) {
					item.BatchStatus = "已过期"
					// 已过期时：查询过期扣减的那笔金额 = 过期那一刻的剩余金额，用于前端展示「过期时还剩多少」
					// 一个充值批次最多对应一条过期扣减记录
					var expiredDeduct float64
					_ = l.svcCtx.DB.QueryRowCtx(l.ctx, &expiredDeduct,
						`SELECT COALESCE((SELECT ABS(xlcredit_amount) FROM ae_user_recharge_record WHERE related_recharge_id = $1 AND xlcredit_amount < 0 LIMIT 1), 0)`, row.Id)
					item.RemainingAtExpire = expiredDeduct
				} else if item.RemainingAmount <= 0 {
					item.BatchStatus = "已耗尽"
				} else {
					item.BatchStatus = "使用中"
				}
			} else {
				item.ExpireTime = "永久有效"
				if item.RemainingAmount <= 0 {
					item.BatchStatus = "已耗尽"
				} else {
					item.BatchStatus = "使用中"
				}
			}
		}

		records = append(records, item)
	}

	return &types.FundChangeRecordResp{
		List:  records,
		Total: total,
	}, nil
}
