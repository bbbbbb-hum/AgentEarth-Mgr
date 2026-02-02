package userfund

import (
	"context"
	"fmt"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/models/users"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ExpirationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExpirationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExpirationLogic {
	return &ExpirationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ProcessExpirationLogic 执行过期余额清理的定时任务逻辑
// 逻辑说明:
// 1. 查找所有已过期且充值金额大于0的记录
// 2. 对每条记录单独计算实时余额
// 3. 如果有剩余余额，插入一条负值的充值记录进行"没收"
// 4. 每条记录处理独立事务，互不影响
func (l *ExpirationLogic) ProcessExpirationLogic(checkTime time.Time) error {
	_ = checkTime
	// 1. 找出所有已过期的正向充值记录
	// 注意: 只能在 SQL 层面筛选出肯定需要检查的记录 (正向充值且时间已过)
	// 至于"剩余余额是否大于0"，需要后续逐条精确计算
	query := `
		SELECT id, user_id, xlcredit_amount, pay_time, create_time, update_time, charge_source, charge_type, remark, operator, expire_time, related_parent_id
		FROM ae_user_recharge_record
		WHERE expire_time < NOW() AND xlcredit_amount > 0
	`

	var expiredRecords []users.AeUserRechargeRecord
	err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &expiredRecords, query)
	if err != nil {
		l.Logger.Errorf("[ProcessExpirationLogic] Failed to query expired records: %v", err)
		return err
	}

	if len(expiredRecords) == 0 {
		l.Logger.Infof("[ProcessExpirationLogic] 未发现需要处理的过期充值记录")
		return nil
	}

	l.Logger.Infof("[ProcessExpirationLogic] Found %d expired records to check", len(expiredRecords))

	// 2. 遍历处理 (单条失败不影响其他)
	deductedCount := 0
	for _, record := range expiredRecords {
		deducted, procErr := l.processSingleRecord(record)
		if procErr != nil {
			// 仅记录错误，不阻断循环
			l.Logger.Errorf("[ProcessExpirationLogic] Failed to process record %d: %v", record.Id, procErr)
			continue
		}
		if deducted {
			deductedCount++
		}
	}

	l.Logger.Infof("[ProcessExpirationLogic] Summary: expired_records=%d, deducted=%d", len(expiredRecords), deductedCount)
	return nil
}

// processSingleRecord 处理单条过期记录的子逻辑
func (l *ExpirationLogic) processSingleRecord(record users.AeUserRechargeRecord) (bool, error) {
	// 开启独立事务
	deducted := false
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// --- 现算余额 ---
		// 复用 balance_helper.go 中的计算逻辑
		initialAmount := decimal.NewFromFloat(record.XlcreditAmount)
		balance, err := CalculateRealTimeBalance(ctx, session, record.Id, initialAmount)
		if err != nil {
			return err
		}

		// --- 判断 ---
		if balance.LessThanOrEqual(decimal.Zero) {
			// 余额为0或负数，说明已经花光了，无需处理
			return nil
		}

		// --- 落库：在充值表插入负值记录 (过期扣减) ---
		// 构造负值金额
		negativeAmount := balance.Neg()

		// 准备 SQL 语句
		insertQuery := `
			INSERT INTO ae_user_recharge_record (
				user_id,
				xlcredit_amount,
				pay_time,
				create_time,
				update_time,
				charge_source,
				charge_type,
				remark,
				related_parent_id,
				operator
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`

		now := time.Now()
		remark := fmt.Sprintf("充值记录 %d 到期自动清理", record.Id)
		// ChargeSource: -2 (假设 -2 代表过期回收，根据之前讨论) 或者沿用 -1
		// ChargeType: 4 (过期扣减)
		chargeSource := -2 // -2 代表过期回收
		chargeType := 4
		operatorName := "System_Auto"

		// 数据库 xlcredit_amount 字段类型为 numeric(20,8)
		// Model 中 XlcreditAmount 被定义为 int64，这可能是 goctl 生成时的映射问题
		// 但数据库层完全支持高精度数值。
		// 此处直接将 decimal 类型的 negativeAmount 传给数据库驱动，
		// pq 驱动会自动处理 decimal 到 numeric 的转换，确保精度不丢失。

		_, err = session.ExecCtx(ctx, insertQuery,
			record.UserId,
			negativeAmount, // 传入 Decimal，pq 驱动支持写入 numeric 字段
			now,            // pay_time
			now,            // create_time
			now,            // update_time
			chargeSource,   // charge_source
			chargeType,     // charge_type
			remark,         // remark
			record.Id,      // related_parent_id
			operatorName,   // operator
		)

		if err != nil {
			return err
		}

		l.Logger.Infof("[ProcessExpirationLogic] 核销详情: 用户=%s, 过期扣减金额=%s, 充值批次ID=%d, 批次过期时间=%s, 操作时间=%s, 备注=%s",
			record.UserId, balance.String(), record.Id, record.ExpireTime.Time.Format(time.RFC3339), now.Format(time.RFC3339), remark)

		deducted = true
		return nil
	})

	if err != nil {
		return false, err
	}

	// 没有错误，且进入扣减分支才会到这里
	return deducted, nil
}
