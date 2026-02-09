# 用户资金核销与余额逻辑关联分析

本文档综合 Mgr 管理员扣减、Stat 日结核销、Stat 过期扣减、balance_helper 及前端展示，检查是否存在逻辑错误。

---

## 一、核心公式与数据源

### 1. 单条充值记录实时余额（balance_helper）

**Mgr 与 Stat 公式一致：**

```
某条充值记录余额 = initialAmount
  - SUM(ae_recharge_allocation.deducted_amount WHERE recharge_record_id = 该条)
  - SUM(ABS(ae_user_recharge_record.xlcredit_amount) WHERE related_recharge_id = 该条 AND xlcredit_amount < 0)
```

- **消费核销**：写入 `ae_recharge_allocation`，通过 `recharge_record_id` 关联批次。
- **负值扣减**（过期扣减 charge_type=4、管理员扣减 charge_type=5 等）：写入 `ae_user_recharge_record` 负值，通过 `related_recharge_id` 关联批次。

因此：**只要负值记录带了 related_recharge_id，就会被正确计入该批次的余额扣减**。两处 balance_helper 逻辑一致，无错误。

---

## 二、用户总余额（快照 + 增量）

**Stat 日结核销与 Mgr 管理员扣减使用同一套公式：**

- **快照**：`ae_user_balance_statistic_daily` 中该用户最近一天的 balance。
- **增量**（快照日次日 00:00 至今）：
  - 正向充值合计
  - 减去消费核销合计（ae_recharge_allocation）
  - 减去负向充值记录合计（ae_user_recharge_record, xlcredit_amount < 0）
- **用户总余额 = 快照 + 增量**

该总余额包含：日结核销死磕导致的负批次、消费挂在过期批次上的情况，与 Stat 日结核销、Mgr 管理员扣减的预校验一致，无逻辑错误。

---

## 三、各模块逻辑检查

### 3.1 Stat 日结核销（SettlementJob）

- **候选批次**：消费日当天未过期（expire_time >= 消费日次日 00:00），FEFO。
- **总余额**：快照 + 增量，用于 effectiveBalance 上限，防止多扣。
- **单条余额**：CalculateRealTimeBalance（含 allocation + related_recharge_id 负值）。
- **死磕**：最后一条批次承担剩余待扣，可透支为负。

结论：与 balance_helper、快照+增量一致，无错误。

### 3.2 Stat 过期扣减（ExpirationDeductionJob）

- **范围**：expire_time < NOW() 且 xlcredit_amount > 0 的充值记录。
- **幂等**：已存在 related_recharge_id = 该批次且 charge_type=4 的负值则跳过。
- **扣减额**：CalculateRealTimeBalance 得到余额，插入一条负值，related_recharge_id = 该批次，charge_type=4。

结论：余额公式与 balance_helper 一致，无错误。

### 3.3 Mgr 管理员扣减（ManualDeductionLogic）

- **预校验**：快照 + 增量得到用户总余额；总余额 ≤ 0 或 < 扣减额则拒绝（不允许透支）。
- **候选批次**：仅未过期（expire_time IS NULL OR expire_time > NOW()），FEFO。
- **扣减**：按 FEFO 遍历，每条用 CalculateRealTimeBalance，只扣余额内部分，插入负值带 related_recharge_id，charge_type=5。

结论：与 balance_helper、快照+增量一致；无未过期批次时拒绝，逻辑正确。

### 3.4 前端展示：某条充值记录余额 / 进度条

- **数据来源**：getFundChangeRecordsLogic，对每条**充值记录**（xlcredit_amount > 0）：
  - `RemainingAmount` = CalculateRealTimeBalance(该条 id, initialAmount)，即与 balance_helper 一致。
  - 进度条可视为 RemainingAmount / InitialAmount。
- **已过期批次**：「过期时还剩多少」展示为 RemainingAtExpire，应取**过期扣减任务**写的那笔（charge_type=4）的 ABS(xlcredit_amount)。

**已修复问题**：原先用 `related_recharge_id = 该条 AND xlcredit_amount < 0 LIMIT 1` 未限定 charge_type，若该批次曾先被管理员扣减再过期，可能误取到管理员扣减金额。已改为 **AND charge_type = 4**，只取过期扣减那笔，语义正确。

---

## 四、执行顺序与一致性

| 事件           | 时间/触发   | 对余额的影响 |
|----------------|------------|--------------|
| 用户消费       | 实时       | 记入日消费表，当晚核销才写 allocation |
| 日结核销       | 每日 01:00 | 写 ae_recharge_allocation，可能死磕导致某批次为负 |
| 过期扣减       | 每日 02:00 | 写 ae_user_recharge_record 负值，charge_type=4，related_recharge_id=批次 |
| 管理员扣减     | 实时       | 写 ae_user_recharge_record 负值，charge_type=5，related_recharge_id=批次 |

- 日结核销早于过期扣减，避免过期批次先被清空再被核销导致不一致。
- 管理员扣减与日结核销、过期扣减都通过 allocation 或 related_recharge_id 反映在 balance_helper 中，前后端共用同一套余额计算，一致。

---

## 五、幂等性与消费透支

### 5.1 日结核销（SettlementJob）

- **幂等入口**：事务外先查 `SUM(deducted_amount) WHERE consumption_daily_id = $1`，若 ≥ 消费总额则整条日消费跳过。
- **事务内**：再次查已分摊总额，`amountToDeduct = consumeAmount - allocatedSum`，避免重试从“全额”重算导致多摊。
- **唯一约束**：`INSERT ... ON CONFLICT (consumption_daily_id, recharge_record_id) DO NOTHING`，同一 (日消费, 充值记录) 只会有一条 allocation。
- **冲突时扣减**：当 `rowsAffected == 0`（记录已存在）时，**必须用库中已存在的 `deducted_amount` 扣减** `amountToDeduct`，不能用本次计算的 `actualDeduct`。否则并发或重试时，本进程算出的 actualDeduct 可能与库中不一致，导致 `amountToDeduct` 多减/少减，最终误报「仍有剩余」或漏摊。
- **消费透支（死磕）**：最后一条候选批次无论 effectiveBalance 是否够，都承担全部剩余待扣（`actualDeduct = amountToDeduct`），该批次余额可变为负；只写 allocation，不写负值表。balance_helper 中该批次余额 = 初始 - allocation - 负值，与现有公式一致，无漏洞。

### 5.2 过期扣减（ExpirationDeductionJob）

- **幂等**：单条记录事务内先查 `COUNT(*) WHERE related_recharge_id = $1 AND xlcredit_amount < 0 AND charge_type = 4`，> 0 则跳过。每个批次最多一条 charge_type=4 的过期扣减。
- **透支无关**：只处理余额 > 0 的过期批次，余额 ≤ 0 直接跳过，不产生负值。

### 5.3 管理员扣减（ManualDeductionLogic）

- **事务**：整段逻辑在同一事务内，要么全部插入成功提交，要么全部回滚，无“插一半”的中间态。
- **无请求级幂等**：未使用 request_id，同一接口重复调用会重复扣减；需防重时需在接口层增加幂等键。

---

## 六、结论与已修复项

- **balance_helper**：Mgr 与 Stat 一致，公式正确；消费核销 + 带 related_recharge_id 的负值均被计入单条充值记录余额。
- **用户总余额**：快照+增量在 Stat 与 Mgr 中一致，用于日结核销的 globalBalance 与管理员扣减预校验，含透支与过期挂账，正确。
- **前端进度条 / 剩余额度**：使用 CalculateRealTimeBalance，与 balance_helper 一致，正确。
- **已修复 1**：getFundChangeRecordsLogic 中「过期时还剩多少」查询已限定 **charge_type = 4**，避免与管理员扣减负值混淆。
- **已修复 2**：Stat 日结核销在 ON CONFLICT DO NOTHING 且 rowsAffected==0 时，改为用**库中已存在的 deducted_amount** 扣减 amountToDeduct，保证与 DB 一致，避免并发/重试下 amountToDeduct 错乱或误报「仍有剩余」。

整体上，管理员扣减、日结核销、过期扣减与单条/总余额计算、前端展示之间逻辑一致；消费透支（死磕）仅影响最后一条批次的 allocation，余额公式与幂等处理已覆盖并修复上述两处。
