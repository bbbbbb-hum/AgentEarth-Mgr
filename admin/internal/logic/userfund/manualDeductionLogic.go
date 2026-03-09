/*
 * 文件名称: manualDeductionLogic.go
 * 功能说明: 人工扣减功能（管理员后台扣减）
 *
 * 主要功能:
 * - 按 FEFO 顺序从用户充值批次依次扣减，每条负值记录关联 related_recharge_id，不允许透支
 * - 仅在 ae_user_recharge_record 表插入扣减记录（金额为负数），不修改用户日余额统计表
 * - 日余额由统计系统统一更新
 *
 * 逻辑要点:
 * - 无可用充值批次或总余额不足时直接拒绝，不做透支挂账
 * - 全流程在同一事务内完成，保证原子性；日志详细便于排查幂等与重复执行
 *
 * API路由: POST /manager/api/userfund/user/deduction
 * 请求体: {"user_id": "xxx", "amount": 100.00, "remarks": "违规扣减"}
 */
package userfund

import (
	"context"
	"errors"
	"fmt"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	fundmodel "AgentEarth-Mgr/models/fund"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ManualDeductionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewManualDeductionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManualDeductionLogic {
	return &ManualDeductionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 管理系统人工扣减逻辑流程图见:https://kcnh6cevaeaq.feishu.cn/wiki/CriGwqkejioBSNk7QRccoMjXnHd
func (l *ManualDeductionLogic) ManualDeduction(req *types.ManualDeductionReq) (resp *types.ManualDeductionResp, err error) {
	operatorName := resolveOperatorName(l.ctx, l.svcCtx, req.UserId, 2) // chargeSource=2 表示管理员单次操作，取当前管理员用户名
	targetUsername := resolveTargetUsername(l.ctx, l.svcCtx, req.UserId)

	// 管理员扣减使用请求中的 charge_type（如 341 等），141 为过期扣减专用不可用，无效或未传时默认 341
	chargeTypeAdminDeduction := req.ChargeType
	if chargeTypeAdminDeduction <= 0 || chargeTypeAdminDeduction == 141 {
		chargeTypeAdminDeduction = 341
	}
	// 后台管理员手动扣减：统一使用 charge_source=2（管理员单次操作）
	chargeSource := int64(2)

	// 1. 参数校验：扣减额必须为正数
	if req.Amount <= 0 {
		l.Errorf("[ManualDeduction] 拒绝扣减: user_id=%s, amount=%.2f, 原因=扣减金额必须大于0", req.UserId, req.Amount)
		return &types.ManualDeductionResp{
			Success: false,
			Message: "扣减金额必须大于0",
		}, nil
	}

	//amountToDeduct表示管理员要扣减的余额
	amountToDeduct := decimal.NewFromFloat(req.Amount)
	l.Infof("[ManualDeduction] 开始处理: user_id=%s, 操作用户=%s, 扣减金额=%s, 备注=%s, operator=%s",
		req.UserId, targetUsername, amountToDeduct.String(), req.Remarks, operatorName)

	// 2. 获取当前展示用的余额（仅日志与返回，不参与事务内决策）
	currentBalance, errBalance := l.svcCtx.UserBalanceDailyModel.GetLatestBalance(l.ctx, req.UserId)
	if errBalance != nil {
		l.Logger.Errorf("[ManualDeduction] 查询用户日余额失败(仅展示用): user_id=%s, err=%v", req.UserId, errBalance)
		currentBalance = 0
	}

	//totalDeducted表示管理员实际已经扣减的余额，当前初始化为0
	var totalDeducted decimal.Decimal
	//insertCount表示管理员实际插入的扣减记录数
	var insertCount int
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 事务内统一使用 WithSession 后的 model
		txRechargeModel := l.svcCtx.UserRechargeRecordModel.WithSession(session)
		txBalanceModel := l.svcCtx.UserBalanceDailyModel.WithSession(session)
		// 3. 预校验：快照+增量得到用户总余额，SQL 在 model 层
		var snapshotBal decimal.Decimal
		var deltaStartTime time.Time

		// 再次查询用户余额快照，保证事务内严谨性，强制所有查询在同一数据库事务内
		// 与GetLatestBalance不同的是，会返回快照金额 + 金额日期，sanp这个结构体，算增量需要用到日期
		snap, errSnap := txBalanceModel.QueryLatestBalanceSnapshot(ctx, req.UserId)
		if errSnap != nil {
			l.Errorf("[ManualDeduction] 查询快照失败: user_id=%s, err=%v", req.UserId, errSnap)
			return fmt.Errorf("查询用户余额快照失败: %w", errSnap)
		}
		if snap != nil {
			if val, parseErr := decimal.NewFromString(snap.Balance); parseErr == nil {
				snapshotBal = val
			}
			// 从快照日的下一天开始计算增量
			deltaStartTime = snap.Day.AddDate(0, 0, 1)
		} else {
			// 无快照，视为 0，从最早时间开始全量计算增量
			snapshotBal = decimal.Zero
			deltaStartTime = time.Time{} //把deltaStartTime清空，充值为0(因为没有余额快照，只能从最初开始计算增量)
			l.Infof("[ManualDeduction] 用户 %s 无余额快照，使用全量流水计算总余额", req.UserId)
		}

		//计算快照时间后的增量deltaStr，未核销昨日用户消费前的真实余额 (管理员扣减已实时核销)
		deltaStr, errDelta := txRechargeModel.QueryBalanceDeltaSince(ctx, req.UserId, deltaStartTime)
		if errDelta != nil {
			l.Errorf("[ManualDeduction] 查询增量失败: user_id=%s, err=%v", req.UserId, errDelta)
			return fmt.Errorf("查询用户余额增量失败: %w", errDelta)
		}
		deltaBal := decimal.Zero
		if val, parseErr := decimal.NewFromString(deltaStr); parseErr == nil {
			deltaBal = val
		}

		//计算当前的全局余额
		globalBalance := snapshotBal.Add(deltaBal)

		l.Infof("[ManualDeduction] 余额预校验明细: user_id=%s, snapshot=%s, delta=%s, global=%s",
			req.UserId, snapshotBal.String(), deltaBal.String(), globalBalance.String())

		//余额小于等于0拒绝扣减
		if globalBalance.LessThanOrEqual(decimal.Zero) {
			l.Infof("[ManualDeduction] 拒绝扣减: user_id=%s, 用户总余额=%s, 原因=余额为0或为负不允许扣减", req.UserId, globalBalance.String())
			return errInsufficientBalance{Required: amountToDeduct, Available: globalBalance}
		}

		//余额小于管理员输入参数，拒绝扣减
		if globalBalance.LessThan(amountToDeduct) {
			l.Infof("[ManualDeduction] 拒绝扣减: user_id=%s, 待扣=%s, 用户总余额=%s, 原因=余额不足不允许透支",
				req.UserId, amountToDeduct.String(), globalBalance.String())
			return errInsufficientBalance{Required: amountToDeduct, Available: globalBalance}
		}
		l.Infof("[ManualDeduction] 预校验通过(快照+增量): user_id=%s, 用户总余额=%s, 待扣=%s", req.UserId, globalBalance.String(), amountToDeduct.String())

		// 4. FEFO 查询：未过期正向充值记录，SQL 在 model 层
		candidates, err := txRechargeModel.QueryRechargeCandidatesForDeduction(ctx, req.UserId)
		if err != nil {
			l.Errorf("[ManualDeduction] 查询充值批次失败: user_id=%s, err=%v", req.UserId, err)
			return fmt.Errorf("查询充值批次失败: %w", err)
		}

		if len(candidates) == 0 {
			l.Infof("[ManualDeduction] 拒绝扣减: user_id=%s, 原因=该用户无任何未过期充值批次", req.UserId)
			return errNoRechargeBatch
		}
		l.Infof("[ManualDeduction] 用户 %s 共 %d 条未过期充值批次(FEFO), 待扣减=%s", req.UserId, len(candidates), amountToDeduct.String())

		// 5. 按 FEFO 依次扣减：每条记录只扣其当前余额内部分，余额为 0 则跳过
		totalDeducted = decimal.Zero
		insertCount = 0
		now := time.Now()
		remarkBase := req.Remarks
		if len(remarkBase) == 0 {
			remarkBase = "管理员扣减" //默认备注，方便区分
		}

		for _, rec := range candidates {
			if amountToDeduct.LessThanOrEqual(decimal.Zero) {
				break
			}

			initialAmount := decimal.NewFromFloat(rec.XlcreditAmount)

			//计算 物理余额 = 初始金额 - 已经被allocation分配走的金额 - 关联所有到这条充值的负值扣减的绝对值总和
			balance, errCalc := CalculateRealTimeBalance(ctx, txRechargeModel, rec.Id, initialAmount)
			if errCalc != nil {
				l.Errorf("[ManualDeduction] 计算批次余额失败: recharge_id=%d, err=%v", rec.Id, errCalc)
				return fmt.Errorf("计算批次 %d 余额失败: %w", rec.Id, errCalc)
			}

			if balance.LessThanOrEqual(decimal.Zero) {
				l.Infof("[ManualDeduction] 跳过批次(余额<=0): recharge_id=%d, 余额=%s, 剩余待扣=%s", rec.Id, balance.String(), amountToDeduct.String())
				continue
			}

			// 本批次实际扣减 = min(本批次余额, 剩余待扣)
			//小于等于就走 实际扣减 = 余额 的逻辑
			actualDeduct := balance
			//大于走 实际扣减 = 剩余待扣 的逻辑 (因为balance足够扣减)
			if balance.GreaterThan(amountToDeduct) {
				actualDeduct = amountToDeduct
			}

			//得到一个专门用于写入扣减流水的 负值金额，后面会转成 float64 存到 xlcredit_amount 字段里，表示这是一条“扣减记录”
			negativeAmount := actualDeduct.Neg()
			// 备注统一用 remarkBase，不写「批次x/y」：实际参与扣减的条数可能远小于候选数，避免误导
			remark := remarkBase

			negFloat, _ := negativeAmount.Float64()
			errExec := txRechargeModel.InsertAdminDeductionRecord(ctx, fundmodel.AdminDeductionInsertParams{
				UserId:            req.UserId,
				NegativeAmount:    negFloat,
				Now:               now,
				ChargeSource:      chargeSource,
				ChargeType:        chargeTypeAdminDeduction,
				Remark:            remark,
				RelatedRechargeID: rec.Id,
				Operator:          operatorName,
			})
			if errExec != nil {
				l.Errorf("[ManualDeduction] 插入扣减记录失败: user_id=%s, recharge_id=%d, amount=%s, err=%v",
					req.UserId, rec.Id, actualDeduct.String(), errExec)
				return fmt.Errorf("插入扣减记录失败: %w", errExec)
			}
			insertCount++
			totalDeducted = totalDeducted.Add(actualDeduct)   //累加实际扣减的金额
			amountToDeduct = amountToDeduct.Sub(actualDeduct) //累减剩余待扣的余额

			expireStr := "永久有效"
			if rec.ExpireTime.Valid {
				if rec.ExpireTime.Time.Year() >= 9999 {
					expireStr = "永久有效"
				} else {
					expireStr = rec.ExpireTime.Time.Format(time.RFC3339)
				}
			}
			l.Infof("[ManualDeduction] 核销详情: user_id=%s, 扣减金额=%s, 充值批次ID=%d, 批次过期时间=%s, 剩余待扣=%s, 备注=%s",
				req.UserId, actualDeduct.String(), rec.Id, expireStr, amountToDeduct.String(), remark)
		}

		//理论上不会发生，因为只有总可用余额 > 管理员扣减额，才允许继续
		if amountToDeduct.GreaterThan(decimal.Zero) {
			l.Errorf("[ManualDeduction] 异常: 事务内扣减后仍有剩余未扣, user_id=%s, 剩余=%s, 已扣=%s, 插入条数=%d",
				req.UserId, amountToDeduct.String(), totalDeducted.String(), insertCount)
			return fmt.Errorf("扣减未完成(剩余%s)，可能并发变更，已回滚", amountToDeduct.String())
		}

		//表示“整个扣减流程在事务内顺利完成，可以安全提交”，是“成功分支的结束语”
		return nil
	})

	if err != nil {
		if errors.Is(err, errNoRechargeBatch) {
			return &types.ManualDeductionResp{
				Success: false,
				Message: "该用户无可用充值批次，无法扣减",
			}, nil
		}
		var e errInsufficientBalance
		if errors.As(err, &e) {
			return &types.ManualDeductionResp{
				Success: false,
				Message: fmt.Sprintf("余额不足，当前可用余额 %s，需扣减 %s，不允许透支", e.Available.String(), e.Required.String()),
			}, nil
		}
		l.Errorf("[ManualDeduction] 事务失败: user_id=%s, err=%v", req.UserId, err)
		return &types.ManualDeductionResp{
			Success: false,
			Message: "扣减执行失败: " + err.Error(),
		}, nil
	}

	//新展示余额 = 原展示余额 − 本次实际扣减总额
	newBalanceForDisplay := currentBalance - totalDeducted.InexactFloat64()
	l.Infof("[ManualDeduction] 扣减成功: user_id=%s, 操作用户=%s, 扣减总额=%s, 插入负值条数=%d, 原展示余额=%.2f, 新展示余额=%.2f (日余额由统计系统更新)",
		req.UserId, targetUsername, totalDeducted.String(), insertCount, currentBalance, newBalanceForDisplay)

	return &types.ManualDeductionResp{
		Success:    true,
		Message:    "扣减成功，余额由统计系统更新",
		NewBalance: newBalanceForDisplay,
	}, nil
}

// 业务错误类型：无充值批次
var errNoRechargeBatch = fmt.Errorf("该用户无可用充值批次")

// errInsufficientBalance 余额不足（不允许透支）
type errInsufficientBalance struct {
	Required  decimal.Decimal
	Available decimal.Decimal
}

func (e errInsufficientBalance) Error() string {
	return fmt.Sprintf("余额不足: 需扣减 %s, 可用 %s", e.Required.String(), e.Available.String())
}
