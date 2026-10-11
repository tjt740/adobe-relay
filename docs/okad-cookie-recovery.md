# Adobe Cookie 自动恢复

当 Sub2 收到 Adobe 401，或 IMS 换 token 确认 Cookie 无效时，使用 Okad 外部子号的同一邮箱重新登录并更新 Cookie。后台在启动时和每轮结束后的扫描周期检查历史鉴权错误；扫描间隔为 30 秒，单账号失败后至少间隔 5 分钟重试。未配置接口时维持原行为。

在服务器持久化的 `gateway` 配置中设置：

- `okad_cookie_refresh_url`：可信 Okad 的 `/api/v1/adobe/cookie/refresh` 地址。
- `okad_cookie_refresh_api_key`：与 Okad 对外接口密钥一致，使用受限配置或密钥管理，不提交到仓库。
- `okad_cookie_refresh_timeout_seconds`：默认 300 秒。

容器内推荐使用共同 Docker 网络的内部地址；跨主机使用 HTTPS。客户端不跟随重定向，日志不记录 Cookie、密钥或 Okad 响应正文。Okad 必须已保存对应外部子号的登录资料。

恢复仅匹配 Adobe OAuth 的明确鉴权错误，排除代理、额度、过期和停用状态。按邮箱校验身份；Cookie、身份、状态、调度开关或代理在登录期间发生变化时，旧恢复不会覆盖新状态。更新保留分组、代理、模型映射及其他配置，清掉旧 token，写入调度更新事件并失效 token 缓存。同进程同账号的并发恢复合并为一次登录。

Okad 的既有异步推送也可先于同步响应到达。原生重新授权清错现在会同时恢复 Adobe 鉴权隔离关闭的调度；其他原因造成的暂停保持原调度设置。代理自动分配仍可能因节点容量不足暂停账号，此时错误信息是代理等待，不能视为 Cookie 恢复失败。

验证：

```sh
cd backend
go test -tags unit ./internal/service -run TestOkadRecovery -count=1
OKAD_RECOVERY_TEST_DATABASE_URL='postgres://USER@127.0.0.1:PORT/DB?sslmode=disable' \
  go test ./internal/repository -run 'TestOkadRecoveryPostgres|TestRecoverAdobeCookieUses' -count=1
```

PostgreSQL 测试需使用独立测试数据库，会创建并清理独立 schema。线上验收应查看 `adobe_okad_cookie_recovered` 日志，再主动查询账号额度验证凭据，同时核对账号状态与调度状态。
