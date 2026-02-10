package fund


import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeUserRechargeRecordModel = (*customAeUserRechargeRecordModel)(nil)

type (
	// AeUserRechargeRecordModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeUserRechargeRecordModel.
	AeUserRechargeRecordModel interface {
		aeUserRechargeRecordModel
		withSession(session sqlx.Session) AeUserRechargeRecordModel
		WithSession(session sqlx.Session) AeUserRechargeRecordModel
		Sum24hRecharge(ctx context.Context) (float64, error)
		GetBalance(ctx context.Context, userId string) (float64, error)

		// fund：资金变动明细 / 扣减 / 余额增量 / 实时余额 / 管理员扣减等
		CountFundChangeRawRows(ctx context.Context, whereClause string, args []interface{}) (int64, error)
		CountMergeableDeductionRows(ctx context.Context, whereClause string, args []interface{}) (int64, error)
		CountMergeableDeductionGroups(ctx context.Context, whereClause string, args []interface{}) (int64, error)
		QueryFundChangeRows(ctx context.Context, whereClause string, baseArgs []interface{}, limit, offset int64) ([]FundChangeRecordRow, error)
		GetExpiredDeductionAmount(ctx context.Context, rechargeID int64) (float64, error)

		InsertRechargeRecordWithExpire(ctx context.Context, userId string, amount float64, payTime, createTime, updateTime time.Time, chargeType int64, remark, operator string, expireTime sql.NullTime) (int64, error)
		InsertRechargeRecordWithoutExpire(ctx context.Context, userId string, amount float64, payTime, createTime, updateTime time.Time, chargeType int64, remark, operator string) (int64, error)

		QueryBalanceDeltaSince(ctx context.Context, userId string, from time.Time) (string, error)
		QueryRechargeCandidatesForDeduction(ctx context.Context, userId string) ([]RechargeRecordForDeduction, error)
		InsertAdminDeductionRecord(ctx context.Context, p AdminDeductionInsertParams) error
		CalculateRealTimeBalance(ctx context.Context, recordID int64, initialAmount decimal.Decimal) (decimal.Decimal, error)
	}

	customAeUserRechargeRecordModel struct {
		*defaultAeUserRechargeRecordModel
	}
)

// NewAeUserRechargeRecordModel returns a model for the database table.
func NewAeUserRechargeRecordModel(conn sqlx.SqlConn) AeUserRechargeRecordModel {
	return &customAeUserRechargeRecordModel{
		defaultAeUserRechargeRecordModel: newAeUserRechargeRecordModel(conn),
	}
}

func (m *customAeUserRechargeRecordModel) withSession(session sqlx.Session) AeUserRechargeRecordModel {
	return NewAeUserRechargeRecordModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAeUserRechargeRecordModel) WithSession(session sqlx.Session) AeUserRechargeRecordModel {
	return m.withSession(session)
}

func (m *customAeUserRechargeRecordModel) Sum24hRecharge(ctx context.Context) (float64, error) {
	// 统计24小时内所有用户充值的总和
	// 使用 pay_time 字段，只统计正值（充值），排除负值（扣减）
	// xlcredit_amount > 0 表示充值，< 0 表示扣减
	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(xlcredit_amount), 0) 
		FROM %s 
		WHERE pay_time >= NOW() - INTERVAL '24 HOURS' 
		AND xlcredit_amount > 0
	`, m.table)
	var total float64
	err := m.conn.QueryRowCtx(ctx, &total, query)
	return total, err
}

func (m *customAeUserRechargeRecordModel) GetBalance(ctx context.Context, userId string) (float64, error) {
	query := fmt.Sprintf("select COALESCE(SUM(xlcredit_amount), 0) from %s where user_id = $1", m.table)
	var balance float64
	err := m.conn.QueryRowCtx(ctx, &balance, query, userId)
	return balance, err
}

// FundChangeRecordRow 对应 ae_user_recharge_record 表（含操作人名称），
// 用于“资金变动明细”列表的数据载体。
// 仅做数据映射，不包含任何业务字段衍生逻辑。
type FundChangeRecordRow struct {
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

// CountFundChangeRawRows 统计满足 where 条件的原始充值/扣减记录总数。
// 说明：
//   - 这里的总数不做“可合并扣减”处理，对应原始行数 totalRaw。
//   - whereClause 由上层业务构建（包含 user_id、金额符号、充值类型等逻辑条件），
//     本方法只负责安全地拼接到 WHERE 子句中。
func (m *customAeUserRechargeRecordModel) CountFundChangeRawRows(ctx context.Context, whereClause string, args []interface{}) (int64, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM ae_user_recharge_record r
		LEFT JOIN ae_mgrsystem_user u ON u.user_id::text = r.operator
		WHERE %s
	`, whereClause)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, query, args...); err != nil {
		return 0, err
	}
	return total, nil
}

// CountMergeableDeductionRows 统计“可合并扣减”（负值且非过期扣减）的原始行数。
// 用于后续按“原始行数 - 可合并行数 + 合并组数”计算展示总条数。
func (m *customAeUserRechargeRecordModel) CountMergeableDeductionRows(ctx context.Context, whereClause string, args []interface{}) (int64, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM ae_user_recharge_record r
		WHERE %s AND r.xlcredit_amount < 0 AND r.charge_type != 4
	`, whereClause)
	var n int64
	if err := m.conn.QueryRowCtx(ctx, &n, query, args...); err != nil {
		return 0, err
	}
	return n, nil
}

// CountMergeableDeductionGroups 统计“可合并扣减”的合并后分组数。
// 分组键与业务层一致：pay_time(秒) + operator + charge_type + remark，避免依赖可能被重写的 create_time。
func (m *customAeUserRechargeRecordModel) CountMergeableDeductionGroups(ctx context.Context, whereClause string, args []interface{}) (int64, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM (
			SELECT 1
			FROM ae_user_recharge_record r
			WHERE %s AND r.xlcredit_amount < 0 AND r.charge_type != 4
			GROUP BY date_trunc('second', r.pay_time), r.operator, r.charge_type, COALESCE(r.remark, '')
		) t
	`, whereClause)
	var n int64
	if err := m.conn.QueryRowCtx(ctx, &n, query, args...); err != nil {
		return 0, err
	}
	return n, nil
}

// QueryFundChangeRows 按 where 条件查询充值/扣减记录，附带管理员名称。
// 特点：
// - 仅负责 SQL 查询与扫描，不做任何“合并扣减”或业务含义推导。
// - 为了后续在业务层做合并展示，由调用方控制 limit/offset，可多取一些行。
func (m *customAeUserRechargeRecordModel) QueryFundChangeRows(ctx context.Context, whereClause string, baseArgs []interface{}, limit, offset int64) ([]FundChangeRecordRow, error) {
	// 现有 whereClause 中已经使用了若干 $n 占位符，这里基于参数长度推导新的下标：
	// - LIMIT 使用 $len+1
	// - OFFSET 使用 $len+2
	argIdx := len(baseArgs) + 1
	query := fmt.Sprintf(`
		SELECT 
			r.id, r.create_time, r.pay_time, r.xlcredit_amount, r.charge_source, r.charge_type,
			r.remark, r.expire_time, COALESCE(u.username, r.operator) as operator
		FROM ae_user_recharge_record r
		LEFT JOIN ae_mgrsystem_user u ON u.user_id::text = r.operator
		WHERE %s
		ORDER BY r.pay_time DESC, r.id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args := make([]interface{}, 0, len(baseArgs)+2)
	args = append(args, baseArgs...)
	args = append(args, limit, offset)
	var rows []FundChangeRecordRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

// GetExpiredDeductionAmount 查询指定充值批次上，与“过期扣减”(charge_type=4) 关联的扣减金额（绝对值）。
// 业务含义：
// - 一条充值记录在过期时，会插入一条负值记录，related_recharge_id 指向原充值记录，charge_type=4。
// - 该负值记录的绝对值代表“过期那一刻的剩余额度”，可用于前端展示「过期时还剩多少」。
// - 如果不存在对应记录，则返回 0。
func (m *customAeUserRechargeRecordModel) GetExpiredDeductionAmount(ctx context.Context, rechargeID int64) (float64, error) {
	const query = `
		SELECT COALESCE((
			SELECT ABS(xlcredit_amount)
			FROM ae_user_recharge_record
			WHERE related_recharge_id = $1 AND xlcredit_amount < 0 AND charge_type = 4
			LIMIT 1
		), 0)
	`
	var amount float64
	if err := m.conn.QueryRowCtx(ctx, &amount, query, rechargeID); err != nil {
		return 0, err
	}
	return amount, nil
}

// InsertRechargeRecordWithExpire 在 ae_user_recharge_record 表中插入一条带过期时间的充值记录。
// 典型场景：管理员人工充值，允许指定某个日期 23:59:59 作为过期时间。
// 注意：
// - charge_source 在业务中固定为 1（后台充值），此处由调用方控制 chargeType、remark、operator 等字段。
func (m *customAeUserRechargeRecordModel) InsertRechargeRecordWithExpire(
	ctx context.Context,
	userId string,
	amount float64,
	payTime, createTime, updateTime time.Time,
	chargeType int64,
	remark, operator string,
	expireTime sql.NullTime,
) (int64, error) {
	const insertQuery = `
		INSERT INTO public.ae_user_recharge_record (
			user_id, xlcredit_amount, pay_time, create_time, update_time,
			charge_source, charge_type, remark, operator, expire_time
		)
		VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $8, $9)
		RETURNING id
	`
	var id int64
	if err := m.conn.QueryRowCtx(ctx, &id, insertQuery, userId, amount, payTime, createTime, updateTime, chargeType, remark, operator, expireTime); err != nil {
		return 0, err
	}
	return id, nil
}

// InsertRechargeRecordWithoutExpire 在 ae_user_recharge_record 表中插入一条“永久有效”的充值记录（不写入 expire_time 字段）。
// 该方法通常作为 InsertRechargeRecordWithExpire 的降级方案：
// - 当数据库 schema 暂未包含 expire_time 字段时，可通过该函数兼容旧环境。
func (m *customAeUserRechargeRecordModel) InsertRechargeRecordWithoutExpire(
	ctx context.Context,
	userId string,
	amount float64,
	payTime, createTime, updateTime time.Time,
	chargeType int64,
	remark, operator string,
) (int64, error) {
	const insertQuery = `
		INSERT INTO public.ae_user_recharge_record (
			user_id, xlcredit_amount, pay_time, create_time, update_time,
			charge_source, charge_type, remark, operator
		)
		VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $8)
		RETURNING id
	`
	var id int64
	if err := m.conn.QueryRowCtx(ctx, &id, insertQuery, userId, amount, payTime, createTime, updateTime, chargeType, remark, operator); err != nil {
		return 0, err
	}
	return id, nil
}

// QueryBalanceDeltaSince 计算从指定起始时间（含）开始，到当前为止用户资金的“净增量”。
// 公式：
//   增量 = 正向充值总额
//         - allocation 表上已经分配走的金额
//         - 负值充值记录（系统/管理员扣减）的绝对值总和
// 说明：
// - 返回值使用 string 承接 numeric，方便上层采用 decimal 高精度运算。
func (m *customAeUserRechargeRecordModel) QueryBalanceDeltaSince(ctx context.Context, userId string, from time.Time) (string, error) {
	const deltaQuery = `
		SELECT
			(SELECT COALESCE(SUM(xlcredit_amount), 0) FROM ae_user_recharge_record WHERE user_id = $1 AND xlcredit_amount > 0 AND create_time >= $2)
			- (SELECT COALESCE(SUM(deducted_amount), 0) FROM ae_recharge_allocation alloc JOIN ae_user_recharge_record rec ON alloc.recharge_record_id = rec.id WHERE rec.user_id = $1 AND alloc.create_time >= $2)
			- (SELECT COALESCE(SUM(ABS(xlcredit_amount)), 0) FROM ae_user_recharge_record WHERE user_id = $1 AND xlcredit_amount < 0 AND create_time >= $2)
	`
	var deltaStr string
	if err := m.conn.QueryRowCtx(ctx, &deltaStr, deltaQuery, userId, from); err != nil {
		return "", err
	}
	return deltaStr, nil
}

// RechargeRecordForDeduction 表示 FEFO 扣减过程中的候选充值批次。
// - 仅包含 FEFO 计算所需的字段：id、user_id、原始充值金额、支付时间、过期时间。
type RechargeRecordForDeduction struct {
	Id             int64        `db:"id"`
	UserId         string       `db:"user_id"`
	XlcreditAmount float64      `db:"xlcredit_amount"`
	PayTime        time.Time    `db:"pay_time"`
	ExpireTime     sql.NullTime `db:"expire_time"`
}

// QueryRechargeCandidatesForDeduction 查询用户所有“未过期、正向充值”的批次，按 FEFO 规则排序：
// - 先按 expire_time 升序（NULLS LAST：永久有效排在最后）
// - 再按 pay_time 升序
func (m *customAeUserRechargeRecordModel) QueryRechargeCandidatesForDeduction(ctx context.Context, userId string) ([]RechargeRecordForDeduction, error) {
	const query = `
		SELECT id, user_id, xlcredit_amount, pay_time, expire_time
		FROM ae_user_recharge_record
		WHERE user_id = $1 AND xlcredit_amount > 0 AND (expire_time IS NULL OR expire_time > NOW())
		ORDER BY expire_time ASC NULLS LAST, pay_time ASC
	`
	var list []RechargeRecordForDeduction
	if err := m.conn.QueryRowsCtx(ctx, &list, query, userId); err != nil {
		return nil, err
	}
	return list, nil
}

// AdminDeductionInsertParams 封装插入管理员扣减记录所需的字段。
type AdminDeductionInsertParams struct {
	UserId            string
	NegativeAmount    float64
	Now               time.Time
	ChargeSource      int64
	ChargeType        int64
	Remark            string
	RelatedRechargeID int64
	Operator          string
}

// InsertAdminDeductionRecord 在 ae_user_recharge_record 中插入一条管理员扣减记录（负值），并关联到具体充值批次。
// 业务约定：
// - xlcredit_amount 为负值（NegativeAmount 传入绝对值的负数）。
// - related_recharge_id 指向被扣减的充值记录 id。
func (m *customAeUserRechargeRecordModel) InsertAdminDeductionRecord(ctx context.Context, p AdminDeductionInsertParams) error {
	const insertQuery = `
		INSERT INTO ae_user_recharge_record (
			user_id, xlcredit_amount, pay_time, create_time, update_time,
			charge_source, charge_type, remark, related_recharge_id, operator
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := m.conn.ExecCtx(ctx, insertQuery,
		p.UserId, p.NegativeAmount, p.Now, p.Now, p.Now,
		p.ChargeSource, p.ChargeType, p.Remark, p.RelatedRechargeID, p.Operator,
	)
	return err
}

// CalculateRealTimeBalance 计算指定充值记录的实时余额。
// 公式：
//   当前余额 = initialAmount
//           - allocation 表上已使用金额
//           - 负值充值记录（related_recharge_id 关联到该批次）的绝对值总和。
// 供 logic 层 balance_helper 统一调用，SQL 仅保留在 model 层。
func (m *customAeUserRechargeRecordModel) CalculateRealTimeBalance(ctx context.Context, recordID int64, initialAmount decimal.Decimal) (decimal.Decimal, error) {
	const query = `
		SELECT
			$2::numeric
			- COALESCE((SELECT SUM(deducted_amount) FROM ae_recharge_allocation WHERE recharge_record_id = $1), 0)
			- COALESCE((SELECT SUM(ABS(xlcredit_amount)) FROM ae_user_recharge_record WHERE related_recharge_id = $1 AND xlcredit_amount < 0), 0)
	`
	var currentBalanceStr string
	if err := m.conn.QueryRowCtx(ctx, &currentBalanceStr, query, recordID, initialAmount.String()); err != nil {
		return decimal.Zero, fmt.Errorf("计算充值记录 %d 的实时余额失败: %w", recordID, err)
	}
	currentBalance, err := decimal.NewFromString(currentBalanceStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("解析充值记录 %d 的余额结果失败: %w", recordID, err)
	}
	return currentBalance, nil
}

