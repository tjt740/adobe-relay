# Adobe 账号自动切换代理

入口：账号管理 → 编辑 Adobe OAuth 账号 → 代理 → 自动故障切换。

先保存账号使用的代理，再开启自动切换并按优先级添加 1–8 个备用代理。点击「保存切换策略」立即生效，策略独立于账号编辑表单保存。既支持普通代理，也支持导入的 Clash 节点。

## 行为

- 每 30 秒检查一次；多个账号共用的代理合并检测，最多 8 个并发，单次超时 6 秒。
- 连续两次连接失败后，选择策略中按优先级排列、最近 45 秒内通过 Adobe 检查的可用代理。检测记录与代理配置绑定，修改配置后需要重新检测。
- 主代理恢复时不会主动切回。当前备用代理再次故障时，重新按主代理、备用列表顺序选择健康出口。
- 没有健康候选时保持当前代理，显示等待备用，不回退直连。启用此策略的账号由此策略接管代理到期后的切换。
- 手动把账号代理改到策略以外，或账号停用、停止调度、过期时暂停检查与切换。重新配置策略或恢复账号后继续。
- 切换只影响后续请求；不重放生图提交，也不能迁移进行中的连接。

检查通过代理向 Adobe Firefly 发出不携带账号令牌的 HEAD 请求，使用与 Adobe 客户端一致的 TLS 浏览器指纹。它验证网络和 TLS 连接，不检查 Cookie、额度，也不能保证后续生图必定成功。429、5xx 和重定向视为暂无法确认，不作为 IP 故障累加。

面板显示当前出口、Adobe 检测延迟、连续失败次数、检查时间及最近一次切换。代理选择器中的普通测速延迟与 Adobe 连接延迟可能不同。后台保留最近 30 天切换记录，管理接口返回最近 10 条。

## 实现与验证

`internal/proxyfailover` 的后台任务用 PostgreSQL 事务级 advisory lock 防止多个服务实例重复切换；更新账号、切换记录和调度 outbox 在同一事务提交。保存策略使用 revision 和当前代理进行并发校验。账号编辑未手动改变代理时，不会提交旧 proxy_id 覆盖后台切换。

管理接口：`GET/PUT /api/v1/admin/accounts/:id/proxy-failover`，沿用管理员鉴权。数据库迁移：`241_account_proxy_failover.sql`。

隔离 PostgreSQL 测试（连接必须指向可创建临时 schema 的测试实例）：

```sh
PROXY_FAILOVER_TEST_DATABASE_URL='postgres://USER:PASS@localhost:15432/postgres?sslmode=disable' \
  go test -race ./internal/proxyfailover ./internal/repository -run 'TestChoose|TestProbe|TestFailover|TestConcurrentChanges|TestExpiryRespectsLiveFailoverPolicy'
```

前端回归：`ProxyFailoverPanel.spec.ts`、`EditAccountModal.spec.ts`、`ProxySelector.testing.spec.ts`。

## 手动 IP 设置

账号编辑中的「手动 IP 设置」随「更新」保存。默认关闭；开启后固定所选代理（无代理为直连），自动分配和自动切换均跳过该账号，检测不临时借用其他出口。绑定仍计入节点容量，也不能作为检测的可回收名额。人工暂停、凭据校验和其他错误保护保持；关闭后恢复原管理规则。明确删除节点或订阅仍执行原清理保护。

管理更新参数为 `manual_proxy_enabled?: boolean`，账号读取为 `extra.manual_proxy_enabled`；迁移 `252_account_manual_proxy.sql` 保留其他原有触发器保护。相关回归见 `manual_proxy_test.go` 及账号编辑测试。发布准备和验证范围见 `deploy/primary/account-manual-ip-20261023.md`，本说明不代表已经部署。

## 408 超时与调度（待发布）

408 只记录失败证据，不再创建持续停调度或必须检测成功才能恢复的门槛，也不单独回收常规代理绑定。跨来源复核仍使用受容量约束的临时租约，结束时归还自己取得的调度资格；原本暂停的账号保持暂停。多账号真实超时仍可触发五分钟节点冷却，到期自动允许普通请求重试，无需成功检测解锁。账号凭据、人工暂停、额度、有效期、订阅流量、永久节点排除和其他限制独立生效。

迁移 `254_adobe_408_timeout_scheduling.sql` 保留失败/成功历史，只按原 408 暂停归属和当前 Cookie 检测证据释放既有门槛。现有账号、分组、凭据和代理选择不由迁移批量重写。发布前应排空旧版本的后台写入，防止其重新建立旧 408 隔离；迁移后不能直接回退旧 408 分配器，以免重新停用账号。当前内容为代码准备，尚未生产部署。
