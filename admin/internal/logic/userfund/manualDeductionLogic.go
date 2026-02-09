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
	"database/sql"
	"errors"
	"fmt"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ManualDeductionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// rechargeRecordForDeduction FEFO 扣减用的充值记录
type rechargeRecordForDeduction struct {
	Id             int64        `db:"id"`
	UserId         string       `db:"user_id"`
	XlcreditAmount float64      `db:"xlcredit_amount"`
	PayTime        time.Time    `db:"pay_time"`
	ExpireTime     sql.NullTime `db:"expire_time"`
}

func NewManualDeductionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManualDeductionLogic {
	return &ManualDeductionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ManualDeductionLogic) ManualDeduction(req *types.ManualDeductionReq) (resp *types.ManualDeductionResp, err error) {
	operatorName := "System_Auto"
	targetUsername := resolveTargetUsername(l.ctx, l.svcCtx, req.UserId)

	// 管理员扣减使用请求中的 charge_type（如 2/3/5 等），4 为过期扣减专用不可用，无效或未传时默认 5
	chargeTypeAdminDeduction := req.ChargeType
	if chargeTypeAdminDeduction <= 0 || chargeTypeAdminDeduction == 4 {
		chargeTypeAdminDeduction = 5
	}
	chargeSource := int64(-1) // 后台

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

	//totalDeducted表示管理员实际扣减的余额
	var totalDeducted decimal.Decimal
	//insertCount表示管理员实际插入的扣减记录数
	var insertCount int
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 3. 预校验：快照+增量得到用户总余额（与 Stat 日结核销一致，含透支、过期批次挂账）
		// 不允许余额为 0 或为负时扣减；总余额不足扣减额时拒绝
		var snapshotBal decimal.Decimal
		var deltaStartTime time.Time

		const snapshotQuery = `
			SELECT balance, day
			FROM ae_user_balance_statistic_daily
			WHERE user_id = $1
			ORDER BY day DESC
			LIMIT 1
		`
		var snap struct {
			Balance string    `db:"balance"`
			Day     time.Time `db:"day"`
		}
		errSnap := session.QueryRowCtx(ctx, &snap, snapshotQuery, req.UserId)
		if errSnap == nil {
			// 找到快照：快照余额 + 从快照次日开始的增量
			if val, parseErr := decimal.NewFromString(snap.Balance); parseErr == nil {
				snapshotBal = val
			}
			deltaStartTime = snap.Day.AddDate(0, 0, 1)
		} else if errSnap == sql.ErrNoRows || errSnap == sqlx.ErrNotFound {
			// 无快照：从“世界初始时间”开始算所有流水，等价于直接用全量流水算真实余额
			snapshotBal = decimal.Zero
			deltaStartTime = time.Time{}
			l.Infof("[ManualDeduction] 用户 %s 无余额快照，使用全量流水计算总余额", req.UserId)
		} else {
			l.Errorf("[ManualDeduction] 查询快照失败: user_id=%s, err=%v", req.UserId, errSnap)
			return fmt.Errorf("查询用户余额快照失败: %w", errSnap)
		}

		const deltaQuery = `
			SELECT
				(SELECT COALESCE(SUM(xlcredit_amount), 0) FROM ae_user_recharge_record WHERE user_id = $1 AND xlcredit_amount > 0 AND create_time >= $2)
				- (SELECT COALESCE(SUM(deducted_amount), 0) FROM ae_recharge_allocation alloc JOIN ae_user_recharge_record rec ON alloc.recharge_record_id = rec.id WHERE rec.user_id = $1 AND alloc.create_time >= $2)
				- (SELECT COALESCE(SUM(ABS(xlcredit_amount)), 0) FROM ae_user_recharge_record WHERE user_id = $1 AND xlcredit_amount < 0 AND create_time >= $2)
		`
		var deltaStr string
		if errDelta := session.QueryRowCtx(ctx, &deltaStr, deltaQuery, req.UserId, deltaStartTime); errDelta != nil {
			l.Errorf("[ManualDeduction] 查询增量失败: user_id=%s, err=%v", req.UserId, errDelta)
			return fmt.Errorf("查询用户余额增量失败: %w", errDelta)
		}
		deltaBal := decimal.Zero
		if val, parseErr := decimal.NewFromString(deltaStr); parseErr == nil {
			deltaBal = val
		}
		globalBalance := snapshotBal.Add(deltaBal) //用户总余额 = 快照余额 + 增量余额
		l.Infof("[ManualDeduction] 余额预校验明细: user_id=%s, snapshot=%s, delta=%s, global=%s",
			req.UserId, snapshotBal.String(), deltaBal.String(), globalBalance.String())

		if globalBalance.LessThanOrEqual(decimal.Zero) {
			l.Infof("[ManualDeduction] 拒绝扣减: user_id=%s, 用户总余额=%s, 原因=余额为0或为负不允许扣减", req.UserId, globalBalance.String())
			return errInsufficientBalance{Required: amountToDeduct, Available: globalBalance}
		}
		if globalBalance.LessThan(amountToDeduct) {
			l.Infof("[ManualDeduction] 拒绝扣减: user_id=%s, 待扣=%s, 用户总余额=%s, 原因=余额不足不允许透支",
				req.UserId, amountToDeduct.String(), globalBalance.String())
			return errInsufficientBalance{Required: amountToDeduct, Available: globalBalance}
		}
		l.Infof("[ManualDeduction] 预校验通过(快照+增量): user_id=%s, 用户总余额=%s, 待扣=%s", req.UserId, globalBalance.String(), amountToDeduct.String())

		// 4. FEFO 查询：仅未过期的正向充值记录，按过期时间升序、支付时间升序（与 Stat 日结核销一致）
		// 过期批次由过期扣减任务回收，不再参与任何扣减
		query := `
			SELECT id, user_id, xlcredit_amount, pay_time, expire_time
			FROM ae_user_recharge_record
			WHERE user_id = $1 AND xlcredit_amount > 0 AND (expire_time IS NULL OR expire_time > NOW())
			ORDER BY expire_time ASC NULLS LAST, pay_time ASC
		`
		var candidates []rechargeRecordForDeduction
		if err := session.QueryRowsCtx(ctx, &candidates, query, req.UserId); err != nil {
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
		if remarkBase == "" {
			remarkBase = "管理员扣减"
		}

		for _, rec := range candidates {
			if amountToDeduct.LessThanOrEqual(decimal.Zero) {
				break
			}

			initialAmount := decimal.NewFromFloat(rec.XlcreditAmount)
			balance, errCalc := CalculateRealTimeBalance(ctx, session, rec.Id, initialAmount)
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

			negativeAmount := actualDeduct.Neg()
			// 备注统一用 remarkBase，不写「批次x/y」：实际参与扣减的条数可能远小于候选数，避免误导
			remark := remarkBase

			insertQuery := `
				INSERT INTO ae_user_recharge_record (
					user_id, xlcredit_amount, pay_time, create_time, update_time,
					charge_source, charge_type, remark, related_recharge_id, operator
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`
			negFloat, _ := negativeAmount.Float64()
			_, errExec := session.ExecCtx(ctx, insertQuery,
				req.UserId, negFloat, now, now, now,
				chargeSource, chargeTypeAdminDeduction, remark, rec.Id, operatorName)
			if errExec != nil {
				l.Errorf("[ManualDeduction] 插入扣减记录失败: user_id=%s, recharge_id=%d, amount=%s, err=%v",
					req.UserId, rec.Id, actualDeduct.String(), errExec)
				return fmt.Errorf("插入扣减记录失败: %w", errExec)
			}
			insertCount++
			totalDeducted = totalDeducted.Add(actualDeduct)   //累加实际扣减的余额
			amountToDeduct = amountToDeduct.Sub(actualDeduct) //累减剩余待扣的余额

			expireStr := "永久有效"
			if rec.ExpireTime.Valid {
				expireStr = rec.ExpireTime.Time.Format(time.RFC3339)
			}
			l.Infof("[ManualDeduction] 核销详情: user_id=%s, 扣减金额=%s, 充值批次ID=%d, 批次过期时间=%s, 剩余待扣=%s, 备注=%s",
				req.UserId, actualDeduct.String(), rec.Id, expireStr, amountToDeduct.String(), remark)
		}

		//理论上不会发生，只有总可用余额 > 扣减额，才允许继续
		if amountToDeduct.GreaterThan(decimal.Zero) {
			l.Errorf("[ManualDeduction] 异常: 事务内扣减后仍有剩余未扣, user_id=%s, 剩余=%s, 已扣=%s, 插入条数=%d",
				req.UserId, amountToDeduct.String(), totalDeducted.String(), insertCount)
			return fmt.Errorf("扣减未完成(剩余%s)，可能并发变更，已回滚", amountToDeduct.String())
		}

		return nil
	})

	if err != nil {
		if errorsAsErrNoRechargeBatch(err) {
			return &types.ManualDeductionResp{
				Success: false,
				Message: "该用户无可用充值批次，无法扣减",
			}, nil
		}
		var e errInsufficientBalance
		if errorsAsErrInsufficientBalance(err, &e) {
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

func errorsAsErrNoRechargeBatch(err error) bool {
	return errors.Is(err, errNoRechargeBatch)
}

func errorsAsErrInsufficientBalance(err error, target *errInsufficientBalance) bool {
	if err == nil || target == nil {
		return false
	}
	var e errInsufficientBalance
	if errors.As(err, &e) {
		*target = e
		return true
	}
	return false
}
