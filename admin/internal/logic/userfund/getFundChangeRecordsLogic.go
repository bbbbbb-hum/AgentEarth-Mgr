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

	// TODO: 需要把多条负值扣减核销记录综合成一条
	// 先查总数（原始行数）
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM ae_user_recharge_record r
		LEFT JOIN ae_mgrsystem_user u ON u.user_id::text = r.operator
		WHERE %s
	`, whereClause)
	var totalRaw int64
	err = l.svcCtx.DB.QueryRowCtx(l.ctx, &totalRaw, countQuery, args...)
	if err != nil {
		l.Logger.Errorf("查询充值记录总数失败...: %v", err)
		return &types.FundChangeRecordResp{List: []types.FundChangeRecordItem{}, Total: 0}, nil
	}
	// 可合并的扣减（负值且非过期扣减）多行合并为一条展示，总条数按“展示条数”算
	total := totalRaw
	if req.Filter != "recharge" {
		var mergeableRows, mergeableGroups int64
		_ = l.svcCtx.DB.QueryRowCtx(l.ctx, &mergeableRows,
			fmt.Sprintf("SELECT COUNT(*) FROM ae_user_recharge_record r WHERE %s AND r.xlcredit_amount < 0 AND r.charge_type != 4", whereClause), args...)
		_ = l.svcCtx.DB.QueryRowCtx(l.ctx, &mergeableGroups,
			fmt.Sprintf("SELECT COUNT(*) FROM (SELECT 1 FROM ae_user_recharge_record r WHERE %s AND r.xlcredit_amount < 0 AND r.charge_type != 4 GROUP BY date_trunc('second', r.create_time), r.operator) t", whereClause), args...)
		total = totalRaw - mergeableRows + mergeableGroups
	}

	// 查询充值记录表（包含充值和扣减），并关联管理员表获取用户名
	// 可合并的扣减（负值且非过期扣减）可能一次操作对应多行，需多取一些行以便合并后仍能填满一页
	limitArgs := append(args, (offset+pageSize)*3, 0) // 多取一些，合并后再分页
	query := fmt.Sprintf(`
		SELECT 
			r.id,
			r.create_time,
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
		ORDER BY r.pay_time DESC, r.id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	type recordRow struct {
		Id             int64          `db:"id"`
		CreateTime     time.Time      `db:"create_time"`
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

	// 将同一次扣减（负值且非过期扣减，charge_type 由管理员指定如 2/3/5 等）的多行合并为一条：按 create_time(秒)+operator 分组
	type groupKey struct {
		Sec      int64
		Operator string
	}

	//map[groupKey][]recordRow 表示：按 groupKey 分组，每组对应多条 recordRow:该组下所有 [可合并的扣减] 的原始行(切片)
	mergeableDeductionGroups := make(map[groupKey][]recordRow)
	for _, row := range rows {
		if row.XlcreditAmount < 0 && row.ChargeType != 4 {
			k := groupKey{row.CreateTime.Unix(), row.Operator.String}
			mergeableDeductionGroups[k] = append(mergeableDeductionGroups[k], row)
		}
	}

	// 每个组中“代表行”取 id 最小的一条（同一次扣减多条里选一条代表）
	groupFirstId := make(map[groupKey]int64) //key是groupKey，value是该组代表行的id(int64)
	for k, list := range mergeableDeductionGroups {
		minId := list[0].Id
		for _, r := range list[1:] { //从第二条开始遍历到最后一行，找到最小的id
			if r.Id < minId {
				minId = r.Id
			}
		}
		groupFirstId[k] = minId //把最小的id赋值给groupFirstId[k]
	}

	var records []types.FundChangeRecordItem
	for _, row := range rows {
		// 可合并的扣减（负值且非过期扣减）：只保留每组代表行，展示为一条合并记录；charge_type 用代表行的值（管理员可指定 2/3/5 等）
		if row.XlcreditAmount < 0 && row.ChargeType != 4 {
			k := groupKey{row.CreateTime.Unix(), row.Operator.String}
			if row.Id != groupFirstId[k] {
				continue
			}
			list := mergeableDeductionGroups[k]
			sumAmount := 0.0
			for _, r := range list {
				sumAmount += r.XlcreditAmount
			}
			operatorName := row.Operator.String
			if operatorName == "" {
				operatorName = "未知"
			}
			remark := row.Remark.String
			if len(list) > 1 {
				remark = fmt.Sprintf("扣减(从%d个批次扣减)", len(list))
			} else if remark == "" {
				remark = chargeTypeDescFromType(row.ChargeType)
			}
			records = append(records, types.FundChangeRecordItem{
				TransactionTime: row.PayTime.Format("2006-01-02 15:04:05"),
				TypeDescription: typeDescFromChargeSource(row.ChargeSource, true),
				ChangeAmount:    sumAmount,
				Status:          "成功",
				Remarks:         remark,
				ChargeType:      row.ChargeType,
				ChargeTypeDesc:  chargeTypeDescFromType(row.ChargeType),
				Operator:        operatorName,
			})
			continue
		}

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
					// 已过期时：优先查询过期扣减(charge_type=4)的那笔金额 = 过期那一刻的剩余金额，用于前端展示「过期时还剩多少」
					// 一个充值批次最多一条过期扣减；需限定 charge_type=4 避免误取管理员扣减等负值
					var expiredDeduct float64
					_ = l.svcCtx.DB.QueryRowCtx(l.ctx, &expiredDeduct,
						`SELECT COALESCE((SELECT ABS(xlcredit_amount) FROM ae_user_recharge_record WHERE related_recharge_id = $1 AND xlcredit_amount < 0 AND charge_type = 4 LIMIT 1), 0)`, row.Id)
					if expiredDeduct > 0 {
						// 已经执行过过期扣减，展示“过期那一刻剩余”
						item.RemainingAtExpire = expiredDeduct
					} else {
						// 还未执行过期扣减：保持状态为已过期，但展示当前实时剩余额度（按照实时余额计算逻辑）
						item.RemainingAtExpire = item.RemainingAmount
					}
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

	// 合并后按展示条数分页
	start := offset
	if start > int64(len(records)) {
		start = int64(len(records))
	}
	end := offset + pageSize
	if end > int64(len(records)) {
		end = int64(len(records))
	}
	list := records[start:end]

	return &types.FundChangeRecordResp{
		List:  list,
		Total: total,
	}, nil
}

// chargeTypeDescFromType 根据 charge_type 返回类型描述（与下方 switch 一致，供合并展示使用）
func chargeTypeDescFromType(chargeType int64) string {
	switch chargeType {
	case 1:
		return "用户常规充值"
	case 2:
		return "系统故障补偿"
	case 3:
		return "活动赠送"
	case 4:
		return "过期扣减"
	case 5:
		return "管理员扣减"
	default:
		return "未知"
	}
}

// typeDescFromChargeSource 根据 charge_source 返回类型说明；isDeduction 为 true 时按扣减语义（管理员后台扣减/系统扣减）
func typeDescFromChargeSource(chargeSource int64, isDeduction bool) string {
	if !isDeduction {
		switch chargeSource {
		case 1:
			return "管理员后台充值"
		case 2:
			return "支付宝充值"
		case 3:
			return "微信充值"
		case 4:
			return "银行卡充值"
		default:
			return "系统充值"
		}
	}
	if chargeSource == -1 {
		return "系统扣减"
	}
	if chargeSource == 1 {
		return "管理员后台扣减"
	}
	return "系统扣减"
}
