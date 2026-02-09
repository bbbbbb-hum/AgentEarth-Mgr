# Apifox 触发统计系统定时任务

用于在 Apifox 中手动触发 AgentEarth-Stat 的**日结核销**与**过期扣减**定时任务，便于 E2E 测试。

## 前置条件

1. 在 Stat 配置中开启测试 API：`cron/etc/cron.yaml` 中设置：
   ```yaml
   TestAPI:
     Enable: true
     Port: 9090
   ```
2. 启动 Stat 服务（如 `go run ./cron -f cron/etc/cron.yaml` 或运行编译后的二进制）。
3. 已按 [test-data-user-e2e4b8b1.sql](./test-data-user-e2e4b8b1.sql) 为测试用户 `e2e4b8b1-f96b-47f2-b6e8-25ab34a81c41` 插入测试数据。

## 接口说明

**两个独立接口**，可分别测试，无需传 body，也无需等间隔时间。

| 接口 | 说明 |
|------|------|
| `POST /test/run-settlement` | 只执行「日结核销」（处理昨日消费） |
| `POST /test/run-expiration` | 只执行「过期扣减」 |

两个接口互不依赖，想测哪个就调哪个，无需先跑核销再等一小时再跑过期。

## Apifox 请求示例

- **日结核销**  
  - Method: `POST`  
  - URL: `http://localhost:9090/test/run-settlement`  
  - Body：无或空均可  

- **过期扣减**  
  - Method: `POST`  
  - URL: `http://localhost:9090/test/run-expiration`  
  - Body：无或空均可  

成功时返回示例：`{"ok":true,"job":"settlement","msg":"已执行"}` 或 `{"ok":true,"job":"expiration","msg":"已执行"}`  

## 管理员扣减

管理员扣减是 **Mgr** 的实时接口，不是 Stat 定时任务。测试管理员扣减请直接调用 Mgr 的「管理员扣减」API，并配合上述测试用户与测试数据（未过期批次、总余额等）进行验证。
