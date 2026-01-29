# 星量用户资金管理模块 - 开发文档

> 创建时间: 2026-01-24  
> 技术栈: Vue.js 3 + Go-Zero + PostgreSQL + ECharts

---

## 📋 目录

1. [功能概述](#功能概述)
2. [系统架构](#系统架构)
3. [数据库设计](#数据库设计)
4. [API接口文档](#api接口文档)
5. [前端页面说明](#前端页面说明)
6. [后端模块说明](#后端模块说明)
7. [部署说明](#部署说明)

---

## 功能概述

### 核心功能

**用户列表页 (UserFundManagement.vue)**
- ✅ 用户列表展示（分页、搜索）
- ✅ 统计数据卡片（总用户、今日活跃、24h充值）
- ✅ 用户卡片（余额、日均消费、动态标签）
- ✅ 卡片悬停炫酷特效
- ✅ 点击跳转详情页

**用户详情页 (UserDetail.vue)**
- ✅ 用户基础信息展示
- ✅ AI资金洞察（余额、续航、日均消费）
- ✅ 余额趋势图（7天/30天曲线图）
- ✅ 资金流向图（消费柱状图）
- ✅ 资金变动明细表（充值/扣减记录）
- ✅ 人工充值功能（两步确认）

---

## 系统架构

```
┌─────────────────────────────────────────────────────┐
│                    前端层 (Vue.js 3)                  │
│  ┌─────────────────────┐  ┌─────────────────────┐   │
│  │ UserFundManagement  │  │    UserDetail       │   │
│  │   (用户列表页)       │──▶│   (用户详情页)      │   │
│  └─────────────────────┘  └─────────────────────┘   │
└──────────────────┬──────────────────────────────────┘
                   │ HTTP/JSON
┌──────────────────▼──────────────────────────────────┐
│               API 网关层 (Go-Zero)                    │
│  ┌──────────────────────────────────────────────┐   │
│  │     userfund.api (7个API端点)               │   │
│  └──────────────────────────────────────────────┘   │
└──────────────────┬──────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────┐
│              业务逻辑层 (Logic)                       │
│  ┌───────────────┐  ┌────────────────┐             │
│  │ getStatsLogic │  │ getUserListLogic│             │
│  ├───────────────┤  ├────────────────┤             │
│  │getUserDetailL │  │getConsumption… │             │
│  ├───────────────┤  ├────────────────┤             │
│  │getBalanceHist │  │getFundChangeR… │             │
│  ├───────────────┤  ├────────────────┤             │
│  │manualRecharge │  │      ...       │             │
│  └───────────────┘  └────────────────┘             │
└──────────────────┬──────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────┐
│              数据访问层 (Model)                       │
│  ┌──────────────────────────────────────────────┐   │
│  │  mcpUserModel                               │   │
│  │  aeUserBalanceStatisticDailyModel           │   │
│  │  aeUserConsumptionRecordDailyModel          │   │
│  │  aeUserRechargeRecordModel                  │   │
│  └──────────────────────────────────────────────┘   │
└──────────────────┬──────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────┐
│               数据库层 (PostgreSQL)                   │
│  ┌──────────────────────────────────────────────┐   │
│  │  ae_user                                    │   │
│  │  ae_user_balance_statistic_daily             │   │
│  │  ae_user_consumption_record_daily            │   │
│  │  ae_user_recharge_record                     │   │
│  └──────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

---

## 数据库设计

### 表结构说明

#### 1. `ae_user` - 用户基础信息表
| 字段 | 类型 | 说明 |
|------|------|------|
| id | int8 | 主键ID |
| user_str_id | text | 用户UUID（唯一） |
| username | varchar(255) | 用户名 |
| email | text | 邮箱 |
| status | varchar(20) | 状态: active/frozen/warning |
| last_login_at | timestamptz | 最后登录时间 |
| create_time | timestamptz | 注册时间 |

#### 2. `ae_user_balance_statistic_daily` - 日余额统计表
| 字段 | 类型 | 说明 |
|------|------|------|
| id | int4 | 主键ID |
| user_str_id | text | 用户UUID |
| day | date | 日期 |
| balance | numeric(16,6) | 余额 |
| create_time | timestamptz | 创建时间 |
| update_time | timestamptz | 更新时间 |

**用途**: 用于余额趋势图、当前余额查询

#### 3. `ae_user_consumption_record_daily` - 日消费记录表
| 字段 | 类型 | 说明 |
|------|------|------|
| id | int4 | 主键ID |
| user_str_id | text | 用户UUID |
| day | date | 日期 |
| xlcredit_consume | int4 | 消费金额（自行消费） |
| create_time | timestamp | 创建时间 |

**用途**: 用于日均消费计算、资金流向柱状图

#### 4. `ae_user_recharge_record` - 充值记录表
| 字段 | 类型 | 说明 |
|------|------|------|
| id | int4 | 主键ID |
| user_str_id | text | 用户UUID |
| xlcredit_amount | numeric(10,2) | 充值金额（负数为扣减） |
| pay_time | timestamptz | 支付时间 |
| charge_source | int2 | 充值方式: 1=后台/2=支付宝/3=微信/4=银行卡 |
| create_time | timestamptz | 创建时间 |
| update_time | timestamptz | 更新时间 |

**用途**: 用于资金变动明细、24h充值统计

---

## API接口文档

### 基础信息
- **Base URL**: `/manager/api/userfund`
- **Content-Type**: `application/json`
- **认证方式**: Cookie (session)

### 接口列表

#### 1. 获取统计数据
```http
GET /manager/api/userfund/stats
```

**响应示例**:
```json
{
  "total_users": 42,
  "daily_active_users": 8,
  "total_recharge_24h": 3500.00
}
```

---

#### 2. 获取用户列表
```http
GET /manager/api/userfund/list?page=1&pageSize=20&search=xxx
```

**Query参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认1 |
| pageSize | int | 否 | 每页条数，默认10 |
| search | string | 否 | 搜索关键词（用户名/邮箱） |

**响应示例**:
```json
{
  "list": [
    {
      "id": 1,
      "user_str_id": "user_10086",
      "username": "陈波良",
      "email": "chen@example.com",
      "status": "active",
      "balance": 5420.50,
      "daily_consumption": 185.00,
      "last_login_at": "2026-01-24T10:00:00Z",
      "create_time": "2023-11-15T08:00:00Z"
    }
  ],
  "total": 42
}
```

---

#### 3. 获取用户详情
```http
GET /manager/api/userfund/user/:user_str_id
```

**响应示例**:
```json
{
  "user": { /* UserItem 结构 */ },
  "current_balance": 5420.50,
  "daily_consumption": 185.00,
  "fund_runway": 29
}
```

---

#### 4. 获取消费记录（资金流向图）
```http
GET /manager/api/userfund/user/:user_str_id/consumption?days=7
```

**Query参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| days | int | 天数: 7 或 30 |

**响应示例**:
```json
{
  "list": [
    {
      "day": "2026-01-18",
      "xlcredit_consume": 120.50
    },
    {
      "day": "2026-01-19",
      "xlcredit_consume": 200.00
    }
  ]
}
```

---

#### 5. 获取余额历史（余额趋势图）
```http
GET /manager/api/userfund/user/:user_str_id/balance?days=7
```

**响应示例**:
```json
{
  "list": [
    {
      "day": "2026-01-18",
      "balance": 5000.00
    },
    {
      "day": "2026-01-19",
      "balance": 5200.00
    }
  ]
}
```

---

#### 6. 获取资金变动明细
```http
GET /manager/api/userfund/user/:user_str_id/fund-changes?filter=all
```

**Query参数**:
| 参数 | 类型 | 说明 |
|------|------|------|
| filter | string | 筛选: all/recharge/deduction |

**响应示例**:
```json
{
  "list": [
    {
      "transaction_time": "2026-01-24 14:20:00",
      "type_description": "支付宝充值",
      "change_amount": 2000.00,
      "status": "成功",
      "remarks": "-"
    },
    {
      "transaction_time": "2026-01-23 10:00:00",
      "type_description": "系统扣减",
      "change_amount": -150.00,
      "status": "成功",
      "remarks": "2023积分过期"
    }
  ]
}
```

---

#### 7. 人工充值
```http
POST /manager/api/userfund/user/recharge
```

**请求体**:
```json
{
  "user_str_id": "user_10086",
  "amount": 100.00,
  "remarks": "活动赠送"
}
```

**响应示例**:
```json
{
  "success": true,
  "message": "充值成功",
  "new_balance": 5520.50
}
```

---

## 前端页面说明

### 页面路由
| 路由 | 组件 | 说明 |
|------|------|------|
| `/user-fund` | UserFundManagement | 用户列表页 |
| `/user-fund/:user_str_id` | UserDetail | 用户详情页 |

### 关键功能实现

#### 1. 日均消费计算
- **数据来源**: 后端自动计算（最近30天平均值）
- **SQL逻辑**: `AVG(xlcredit_consume) WHERE xlcredit_consume > 0`
- **展示位置**: 用户卡片 footer、AI洞察卡片

#### 2. 资金续航预估 (Runway)
- **计算公式**: `续航天数 = 当前余额 / 日均消费`
- **特殊情况**: 日均消费为0时，显示"无限"
- **展示形式**: 数字 + 进度条

#### 3. 余额滚动动画
```typescript
// 实现原理: CountUp动画，2秒内从0滚动到目标值
const animateBalance = () => {
  const duration = 2000; // 2秒
  const steps = 60;
  const increment = targetBalance.value / steps;
  // ... 每33ms更新一次余额显示
}
```

#### 4. 曲线图自动滑动
```typescript
// 实现原理: 进入页面800ms后自动滑动到最新数据点
setTimeout(() => {
  lineChart.value.dispatchAction({
    type: 'dataZoom',
    startValue: Math.max(0, dates.length - 7),
    endValue: dates.length - 1
  });
}, 800);
```

---

## 后端模块说明

### 文件结构
```
admin/
├── api/
│   └── userfund.api              # API定义文件
├── internal/
│   ├── handler/userfund/         # HTTP请求处理器（7个）
│   │   ├── getStatsHandler.go
│   │   ├── getUserListHandler.go
│   │   ├── getUserDetailHandler.go
│   │   ├── getConsumptionRecordsHandler.go
│   │   ├── getBalanceHistoryHandler.go
│   │   ├── getFundChangeRecordsHandler.go
│   │   └── manualRechargeHandler.go
│   ├── logic/userfund/           # 业务逻辑层（7个）
│   │   ├── getStatsLogic.go
│   │   ├── getUserListLogic.go
│   │   ├── getUserDetailLogic.go
│   │   ├── getConsumptionRecordsLogic.go
│   │   ├── getBalanceHistoryLogic.go
│   │   ├── getFundChangeRecordsLogic.go
│   │   └── manualRechargeLogic.go
│   └── types/
│       └── types.go              # 类型定义
└── models/users/
    ├── mcpUserModel.go
    ├── aeUserBalanceStatisticDailyModel.go
    └── aeUserRechargeRecordModel.go
```

### 关键逻辑说明

#### 1. 日均消费计算 (getUserListLogic.go)
```go
// 查询最近30天的消费记录，计算平均值
query := `
    SELECT COALESCE(AVG(xlcredit_consume), 0) as avg_consumption
    FROM ae_user_consumption_record_daily
    WHERE user_str_id = $1
    AND day >= CURRENT_DATE - INTERVAL '30 days'
    AND xlcredit_consume > 0
`
```

#### 2. 资金续航计算 (getUserDetailLogic.go)
```go
// Runway = 当前余额 / 日均消费
if dailyConsumption > 0 {
    fundRunway = int64(currentBalance / dailyConsumption)
} else {
    fundRunway = 9999 // 表示"无限"
}
```

#### 3. 人工充值事务 (manualRechargeLogic.go)
```go
// 步骤1: 插入充值记录
INSERT INTO ae_user_recharge_record ...

// 步骤2: 更新日余额（使用 ON CONFLICT DO UPDATE）
INSERT INTO ae_user_balance_statistic_daily
ON CONFLICT (user_str_id, day)
DO UPDATE SET balance = balance + $amount
```

---

## 部署说明

### 后端启动
```bash
cd d:\workspace\AgentEarth-Mgr\admin
go run admin.go
# 或
go build -o admin.exe admin.go
./admin.exe
```

### 前端启动
```bash
cd d:\workspace\AgentEarth-MgrFE
npm install
npm run dev
```

### 访问地址
- **前端**: http://localhost:5173/manager/user-fund
- **后端API**: http://localhost:9005/manager/api/userfund/*

### 环境要求
- **Go**: 1.20+
- **Node.js**: 18+
- **PostgreSQL**: 14+
- **浏览器**: Chrome 90+ / Edge 90+ / Firefox 88+

---

## 注意事项

### 1. 数据一致性
- 充值操作会同时更新 `ae_user_recharge_record` 和 `ae_user_balance_statistic_daily`
- 使用 `ON CONFLICT DO UPDATE` 确保同一天的余额记录唯一性

### 2. 性能优化
- 用户列表查询包含日均消费计算，建议在数据库层面建立索引
- 大数据量时考虑使用缓存（Redis）

### 3. 安全考虑
- 人工充值有前端二次确认弹窗
- 建议添加后端权限校验（管理员角色）
- 充值记录应保留完整审计日志

### 4. 扩展性
- 如需添加更多图表，可在 `UserDetail.vue` 中扩展 ECharts 配置
- 如需添加更多统计维度，可在 `getStatsLogic.go` 中扩展 SQL 查询

---

## 技术支持

如有问题，请联系开发团队或查阅：
- Go-Zero文档: https://go-zero.dev/
- Vue.js文档: https://vuejs.org/
- ECharts文档: https://echarts.apache.org/

---

**文档更新时间**: 2026-01-24  
**版本**: v1.0.0
