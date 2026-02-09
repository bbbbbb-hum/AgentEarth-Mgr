-- =============================================================================
-- 用户 e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41 全场景测试数据
-- 用途：日结核销(Settlement)、管理员扣减(ManualDeduction)、过期扣减(ExpirationDeduction)
-- 执行前请确认该用户仅在测试环境使用；执行后可用 Apifox 调 Stat 的 /test/run-job 触发定时任务验证。
-- =============================================================================

-- 清理该用户已有测试数据（可选，首次可注释掉）
DELETE FROM ae_recharge_allocation
WHERE consumption_daily_id IN (
  SELECT id FROM ae_user_consumption_record_daily
  WHERE user_id = 'e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41'
);
DELETE FROM ae_user_consumption_record_daily
WHERE user_id = 'e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41';
DELETE FROM ae_user_recharge_record
WHERE user_id = 'e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41';
DELETE FROM ae_user_balance_statistic_daily
WHERE user_id = 'e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41';

-- -----------------------------------------------------------------------------
-- 1. 充值记录（按 pay_time 升序 = FEFO 顺序）
-- -----------------------------------------------------------------------------
-- create_time/update_time 与 pay_time 对齐，避免快照+增量用 create_time 做条件时边界错误
-- R1: 100，永久有效，用于日结核销 + 管理员扣减
-- R2: 50，过期时间「明天」，用于日结核销（消费日次日未过期）+ 幂等预插一条 allocation
-- R3: 30，已过期（昨天），用于过期扣减任务：应插入 -30 的 charge_type=4
-- R4: 20，已过期（前天），且下面会预插 allocation 20，余额=0，过期扣减应跳过
INSERT INTO ae_user_recharge_record (
  user_id, xlcredit_amount, pay_time, create_time, update_time,
  charge_source, charge_type, expire_time
)
VALUES
  ('e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41', 100, (CURRENT_DATE - 5)::timestamp, (CURRENT_DATE - 5)::timestamp, (CURRENT_DATE - 5)::timestamp, 1, 1, NULL),
  ('e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41', 50,  (CURRENT_DATE - 4)::timestamp, (CURRENT_DATE - 4)::timestamp, (CURRENT_DATE - 4)::timestamp, 1, 1, (CURRENT_DATE + 1)::timestamp),
  ('e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41', 30,  (CURRENT_DATE - 3)::timestamp, (CURRENT_DATE - 3)::timestamp, (CURRENT_DATE - 3)::timestamp, 1, 1, (CURRENT_DATE - 1)::timestamp),
  ('e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41', 20,  (CURRENT_DATE - 2)::timestamp, (CURRENT_DATE - 2)::timestamp, (CURRENT_DATE - 2)::timestamp, 1, 1, (CURRENT_DATE - 2)::timestamp);

-- -----------------------------------------------------------------------------
-- 2. 日消费记录
-- -----------------------------------------------------------------------------
-- 昨日消费 80：日结核销会按 FEFO 摊到「消费日次日未过期」的批次（R2、R1），预插 (昨日, R2, 50) 测幂等
-- 3 天前消费 20：已摊到 R4，用于把 R4 余额打成 0，过期扣减时跳过 R4
INSERT INTO ae_user_consumption_record_daily (user_id, day, xlcredit_consume, create_time)
VALUES
  ('e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41', CURRENT_DATE - 1, 80, now()),
  ('e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41', CURRENT_DATE - 3, 20, now());

-- -----------------------------------------------------------------------------
-- 3. 余额快照（供快照+增量算总余额）
-- -----------------------------------------------------------------------------
INSERT INTO ae_user_balance_statistic_daily (user_id, day, balance, create_time, update_time)
VALUES ('e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41', CURRENT_DATE - 3, 200, now(), now());

-- -----------------------------------------------------------------------------
-- 4. 预插分摊记录
-- -----------------------------------------------------------------------------
-- 4.1 (3天前消费, R4, 20)：把 R4 批次扣光，过期扣减任务对 R4 应跳过（余额=0）
INSERT INTO ae_recharge_allocation (consumption_daily_id, recharge_record_id, deducted_amount, create_time, update_time)
SELECT d.id, r.id, 20, now(), now()
FROM ae_user_consumption_record_daily d
CROSS JOIN ae_user_recharge_record r
WHERE d.user_id = 'e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41' AND d.day = (CURRENT_DATE - 3)
  AND r.user_id = 'e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41' AND r.pay_time = (CURRENT_DATE - 2)::timestamp
LIMIT 1;

-- 4.2 (昨日消费, R2, 50)：日结核销幂等测试，重跑时应用库中 50 扣减，再摊 30 到 R1
INSERT INTO ae_recharge_allocation (consumption_daily_id, recharge_record_id, deducted_amount, create_time, update_time)
SELECT d.id, r.id, 50, now(), now()
FROM ae_user_consumption_record_daily d
CROSS JOIN ae_user_recharge_record r
WHERE d.user_id = 'e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41' AND d.day = (CURRENT_DATE - 1)
  AND r.user_id = 'e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41' AND r.pay_time = (CURRENT_DATE - 4)::timestamp
LIMIT 1;

-- =============================================================================
-- 场景对照（执行上述 SQL 后）
-- =============================================================================
-- 日结核销(SettlementJob, 跑「昨日」):
--   - 昨日消费 80，有效批次 FEFO：R2(50), R1(100)。已存在 (昨日,R2,50)，幂等用库中 50；再摊 30 到 R1。
--   - 预期：一条新 allocation (昨日, R1, 30)；重跑不再新增，且用库中 deducted_amount 扣减。
--
-- 过期扣减(ExpirationDeductionJob):
--   - R3: 过期且余额 30 → 插入一条 -30, charge_type=4, related_recharge_id=R3。
--   - R4: 过期但余额 0（已被 allocation 扣光）→ 跳过。
--   - 再次执行：R3 已存在 charge_type=4 负值 → 幂等跳过。
--
-- 管理员扣减(ManualDeduction, Mgr 接口，非 Stat 定时):
--   - 未过期批次：R1, R2。总余额（快照+增量）> 0 时可扣；可测：单批次够扣、多批次 FEFO、余额不足拒绝。
-- =============================================================================
