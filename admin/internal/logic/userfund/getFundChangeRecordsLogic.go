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
 *
 * API路由: GET /manager/api/userfund/user/:user_id/fund-changes?filter=all
 */

package userfund

import (
	"context"
	"fmt"
	"math"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	fundmodel "AgentEarth-Mgr/models/fund"

	"github.com/shopspring/decimal"
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
	//req.ChargeType == 0表示all，不筛选
	if req.ChargeType > 0 {
		whereClause += fmt.Sprintf(" AND r.charge_type = $%d", argIdx)
		args = append(args, req.ChargeType)
		argIdx++
	}

	// 先查总数（原始行数），SQL 在 model 层
	totalRaw, err := l.svcCtx.UserRechargeRecordModel.CountFundChangeRawRows(l.ctx, whereClause, args)
	if err != nil {
		l.Logger.Errorf("查询充值记录总数失败: %v", err)
		return &types.FundChangeRecordResp{List: []types.FundChangeRecordItem{}, Total: 0}, nil
	}
	// 可合并的扣减（负值且非过期扣减）多行合并为一条展示，总条数按“展示条数”算
	total := totalRaw
	if req.Filter != "recharge" {
		mergeableRows, _ := l.svcCtx.UserRechargeRecordModel.CountMergeableDeductionRows(l.ctx, whereClause, args)
		mergeableGroups, _ := l.svcCtx.UserRechargeRecordModel.CountMergeableDeductionGroups(l.ctx, whereClause, args)
		total = totalRaw - mergeableRows + mergeableGroups
	}

	// 查询充值/扣减记录并关联管理员名称
	var limit, queryOffset int64
	if req.Filter == "recharge" {
		// 仅充值：无合并逻辑，直接用 DB 分页，避免后面页展示不全
		limit = pageSize
		queryOffset = offset
	} else {
		// 全部/仅扣减：只有负值且 charge_type!=4 的扣减会在内存中合并，正值（充值）一条就是一条，从不合并。
		// 为凑够 (offset+pageSize) 条展示记录，需多取原始行（因部分原始行合并后展示条数变少）。
		const mergeLimitMultiplier = 3
		const mergeLimitCap = 7000 // 单次最多取 5000 条原始行，后面页的充值/扣减会永远取不到
		limit = (offset + pageSize) * mergeLimitMultiplier
		if limit > mergeLimitCap {
			limit = mergeLimitCap
		}
		queryOffset = 0
	}
	rows, err := l.svcCtx.UserRechargeRecordModel.QueryFundChangeRows(l.ctx, whereClause, args, limit, queryOffset)
	if err != nil {
		l.Logger.Errorf("Failed to query fund change records: %v", err)
		return &types.FundChangeRecordResp{List: []types.FundChangeRecordItem{}, Total: 0}, nil
	}

	// 仅对「负值且 charge_type!=4」的扣减做合并；正值（充值）记录一条就是一条，从不合并。
	// 按 pay_time(秒) + operator + charge_type + remark 分组，同一组的扣减多行合成一条展示。
	type groupKey struct {
		Sec        int64
		Operator   string
		ChargeType int64
		Remark     string
	}

	// 按 groupKey 分组，每组对应多条可合并扣减的原始行
	//key=一次扣减的"特征" value=属于这一批扣减的所有原始行列表
	mergeableDeductionGroups := make(map[groupKey][]fundmodel.FundChangeRecordRow)
	for _, row := range rows {
		if row.XlcreditAmount < 0 && row.ChargeType != 4 {
			k := groupKey{
				Sec:        row.PayTime.Unix(),
				Operator:   row.Operator.String,
				ChargeType: row.ChargeType,
				Remark:     row.Remark.String,
			}
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
			k := groupKey{
				Sec:        row.PayTime.Unix(),
				Operator:   row.Operator.String,
				ChargeType: row.ChargeType,
				Remark:     row.Remark.String,
			}

			//原始行id不在展示组里（每组第一条记录用来展示）
			if row.Id != groupFirstId[k] {
				continue
			}
			list := mergeableDeductionGroups[k]
			sumAmount := 0.0
			for _, r := range list {
				sumAmount += r.XlcreditAmount
			}
			operatorName := row.Operator.String
			if len(operatorName) == 0 {
				operatorName = "未知"
			}
			remark := row.Remark.String
			if len(list) > 1 {
				remark = fmt.Sprintf("扣减(从%d个批次扣减)", len(list))
			} else if len(remark) == 0 {
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

		// 根据金额和 charge_source / charge_type 判断类型文案，复用下方工具函数
		typeDesc := typeDescFromChargeSource(row.ChargeSource, row.XlcreditAmount < 0)
		chargeTypeDesc := chargeTypeDescFromType(row.ChargeType)

		//设置操作人用户名
		operatorName := row.Operator.String
		if len(operatorName) == 0 {
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

		// 仅充值时计算批次剩余额度、过期时间、状态、透支金额
		if row.XlcreditAmount > 0 {
			item.BatchId = row.Id                                  //设置前端展示的批次id = 充值记录在表里的主键id
			item.InitialAmount = row.XlcreditAmount                //初始金额 = 充值金额
			initialAmt := decimal.NewFromFloat(row.XlcreditAmount) //浮点型金额转成高精度decimal，避免浮点误差
			//计算实时剩余额度 = 初始金额 - 消费核销 - 充值表关联负值记录
			remaining, errBalance := CalculateRealTimeBalance(l.ctx, l.svcCtx.UserRechargeRecordModel, row.Id, initialAmt)
			if errBalance == nil {
				//把decimal类型的remaining转成float64，放到返回给前端的item.RemainingAmount字段里
				item.RemainingAmount, _ = remaining.Float64()
			}

			// 先算透支金额（供下面「过期时剩余」判断用）：先透支后过期 → 过期时剩余必须为 0，不能再用过期扣减记录“加回”
			// 业务约定：消费记录只挂载在正向充值记录上，不会挂在过期扣减等负值子记录上；baseOutflow 已含本批次全部核销。
			var overdraftAmt decimal.Decimal //定义透支金额，初始值为0
			if errBalance == nil {           //只有在成功算出remaining时，才继续算透支
				baseOutflow := initialAmt.Sub(remaining) //基础流出代表到当前这一刻为止，已经"流出去"的那部分金额(=初始金额-当前剩余)
				childrenAllocStr, errChild := l.svcCtx.RechargeAllocationModel.GetAllocatedSumByRechargeChildren(l.ctx, row.Id)
				extraChildrenAlloc := decimal.Zero
				if errChild == nil {
					if v, parseErr := decimal.NewFromString(childrenAllocStr); parseErr == nil {
						extraChildrenAlloc = v
					}
				}
				// extraChildrenAlloc 正常为 0（消费不挂子记录），保留用于口径统一与历史数据兼容
				//透支 = 基础流出 + 管理员扣减挂载 - 初始金额
				overdraftAmt = baseOutflow.Add(extraChildrenAlloc).Sub(initialAmt)
				//如果计算结果是负数，说明"流出不足初始金额"，那不算透支
				if overdraftAmt.IsNegative() {
					overdraftAmt = decimal.Zero
				}
				item.OverdraftAmount, _ = overdraftAmt.Float64() //把透支金额转换成 float64，填到 item.OverdraftAmount 里。
				// 供前端展示“这个充值批次透支了多少”
			}

			if row.ExpireTime.Valid {
				item.ExpireTime = row.ExpireTime.Time.Format("2006-01-02")
				now := time.Now()
				if row.ExpireTime.Time.Before(now) {
					item.BatchStatus = "已过期"

					// ==================== 过期时剩余：优先用本批结果里「关联本条充值 id 的负值过期扣减」金额 ====================
					l.Infof("[过期时剩余] 已过期批次 充值批次id=%d 初始金额=%.2f 当前剩余=%.2f 透支金额=%.2f 开始计算过期时剩余",
						row.Id, row.XlcreditAmount, item.RemainingAmount, item.OverdraftAmount)
					var expiredAmount float64 //代表某个充值批次在过期那一刻还剩下多少没有用完【过期时剩余】
					for _, r := range rows {
						//遍历所有原始行rows
						//1.这条记录关联了某个充值id，也就是related_recharge_id不为空
						//2.关联的充值记录id就是当前这条充值id
						//3.charge_type = 过期扣减
						//4.充值金额是负值
						if r.RelatedRechargeId.Valid && r.RelatedRechargeId.Int64 == row.Id && r.ChargeType == 4 && r.XlcreditAmount < 0 {
							//过期时剩余 = abs(负值金额)
							expiredAmount = math.Abs(r.XlcreditAmount)
							l.Infof("[过期时剩余] 从本批结果中找到关联的过期扣减记录 扣减记录id=%d 关联充值id=%d 扣减金额=%.2f => 过期时剩余=%.2f",
								r.Id, r.RelatedRechargeId.Int64, r.XlcreditAmount, expiredAmount)
							break //找到一条有过期的负值扣减记录就够了(实际上正常逻辑下每条充值记录只会有一条)
						}
					}
					if expiredAmount == 0 {
						//可能是由于这一次查询出来的rows原始行记录中数量太少，没有找到关联的过期扣减记录
						//也就是此次分页查询没有包含那条过期扣减行，再次从数据库查一遍
						fromDB, errDB := l.svcCtx.UserRechargeRecordModel.GetExpiredDeductionAmount(l.ctx, row.Id)
						l.Infof("[过期时剩余] 本批未找到扣减行，查库 充值批次id=%d => 查询结果金额=%.2f 错误=%v", row.Id, fromDB, errDB)
						expiredAmount = fromDB
					}
					if expiredAmount > 0 {
						//这就是当前充值批次“过期那一刻的剩余金额”
						item.RemainingAtExpire = expiredAmount
						l.Infof("[过期时剩余] 最终赋值 过期时剩余=%.2f（来自过期扣减金额）", expiredAmount)
					} else {
						//走到这里没有查到任何过期扣减金额，但是这批有已经处于已过期状态
						if item.RemainingAmount > 0 { //当前实时剩余 > 0，定时任务没有跑完
							item.RemainingAtExpire = item.RemainingAmount
							l.Infof("[过期时剩余] 最终赋值 过期时剩余=%.2f（定时任务未跑，用当前剩余）", item.RemainingAmount)
						} else {
							//当前实时剩余 < 0，说明这批要么已经被花光，要么已经被其他方式扣完
							//但是有没有过期扣减记录可参考，那就认为过期时剩余为0
							item.RemainingAtExpire = 0
							l.Infof("[过期时剩余] 最终赋值 过期时剩余=0（无过期扣减记录且当前剩余<=0）")
						}
					}
					// ==================== 修正结束 ====================

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

	// 合并后按展示条数分页（仅充值时已在 DB 分页，records 即当前页）
	var list []types.FundChangeRecordItem
	//仅充值场景不需要合并，在前面查询时已经用sql做了limit/offset，数据库层面已经按页查好了
	if req.Filter == "recharge" {
		list = records //此时records本身就是“当前页”的数据，不需要再在内存里切分页
	} else {
		start := offset //从records的第offset条开始取，因为合并后可能超出要取的条数
		if start > int64(len(records)) { //越界保护，如果offset超出records的长度，把start强行拉到len(records)的位置
			start = int64(len(records))
		}
		end := offset + pageSize //从offset开始取pageSize条
		if end > int64(len(records)) { //如果end超出records的长度，就把end收缩到len(records)的位置
			end = int64(len(records))
		}
		list = records[start:end] //取records的第start条到第end条
	}

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
