# Okad 外部子号错误状态与筛选

迭代：`okad-account-errors-20261014`。用户授权合并 main、推送并发布到主服务器 `173.234.15.150`（`https://creative.cheap`），保留原功能、配置、数据和回滚点。

## 变更

- Sub2 库存协议提供尚未恢复的业务/检测错误及当前错误状态，读取原有被动证据，不发起检测或登录。
- Okad 外部子号展示 401（红）、408（黄）及其他错误码，支持全量分页前筛选、与原条件组合及筛选导出。
- 维持 30 秒库存缓存和页面轮询；手动刷新可重新读取。未知和连接失败不会显示为正常。
- 保留成功证据恢复、人工暂停、Cookie/代理保护、原自动恢复和低频邮件规则。无数据库迁移。

## 发布保护

Sub2 发布前实际镜像为 `020430320d0fbc31258ea21fcdfdd08d10101697`，保留图片 URL 与真实 usage/null 逻辑。发布前主干为 `876e2a35087f08f6ef2af0c0ddc36d0dd604c833`。Okad 主干为 `ef022152a597f1c2b38f9657168c632bad070d85`，实际容器另有已上线 `06d1137` 积分、配置和公司筛选；本次将这部分纳入主干并保留运行行为。

Okad 以实际运行容器为基础，仅覆盖新增状态所需文件；比较全量 Python/JS/CSS/HTML 哈希，保留监控、移动端及其他叠加功能。切换前检查双项目发布锁、增量发布锁和业务任务；不终止活动登录或 Cookie 交付。

服务器审计目录：`/opt/adobe-relay/releases/okad-account-errors-20261014/`。Sub2 正式脚本保存并验证 PostgreSQL 转储、配置和应用数据；Okad 保存 SQLite 一致性备份、配置、宿主源文件及实际运行镜像。所有备份留在服务器，敏感值不写发布日志。

## 回滚

保留镜像 `adobe-relay:before-okad-account-errors-20261014`、`adobe-team-cookie:before-okad-account-errors-20261014`。

只读检查：

```sh
python3 /opt/adobe-relay/releases/okad-account-errors-20261014/account-errors-release.py check-rollback
```

确认需要撤回本次双项目代码时：

```sh
python3 /opt/adobe-relay/releases/okad-account-errors-20261014/account-errors-release.py rollback
```

脚本检查当前镜像、全量 Okad 源码、配置、设置及告警规则；存在后续改动会拒绝覆盖。只恢复应用代码，保留当前数据库、Cookie、代理、SMTP 和业务数据。不要恢复旧数据库，也不要绕过校验。

## 验证

开发阶段相关 Go 测试与 race、隔离 PostgreSQL 证据测试通过；原生产脱敏回放因没有提供快照而跳过。Okad 合并运行筛选后 94 项后端回归及错误状态/原筛选两套浏览器回归通过，覆盖桌面、手机、两个路径、分页和组合筛选。13 项部署保护检查通过。正式流程后端 race/数据库/SMTP、337 项前端回归、TypeScript 和镜像构建通过。

## 实际发布

- Sub2 [PR #48](https://github.com/tjt740/adobe-relay/pull/48) 合并为 `a2b46f5eac2bd04ddf6dc2d08843ec4843a086ad`；[正式工作流 38573830232](https://github.com/tjt740/adobe-relay/actions/runs/38573830232) 成功，北京时间 21:52 切换。
- Okad [PR #5](https://github.com/tjt740/adobe-team-cookie/pull/5) 合并为 `c6e00d968d521d2b157eb0d3470a78faabd0e64d`；[增量发布与核验 38574702116](https://github.com/tjt740/adobe-relay/actions/runs/38574702116) 成功，22:01:34 完成切换。实际镜像 ID 为 `sha256:b3f832191c0ae19d65328e5503546b65e604c0e02042e70c5ebe407b559bb69e`。
- 两仓库 `release/okad-account-errors-20261014` 标签指向各自实际应用提交；`rollback/okad-account-errors-20261014-before-main` 分别指向 `876e2a350` 和 `ef022152a`。归档提交不作为应用镜像版本，不重复部署。
- Okad 仅改变路由、schema、库存服务、新增健康投影、外部子号 JS 和两个 HTML 入口，共 7 个文件；全量源码哈希验证除这 7 个文件外均与原运行容器一致。保留积分/配置/公司筛选、移动端、服务器监控和其他叠加功能，后续仍应以实际容器核对，不能仅凭 Git 标签全量覆盖。
- 等待期间没有活动登录和恢复，持续存在已持久化的未来交付重试。最初将未来 60 秒的重试也计为忙，导致一直等待；审查原持久队列与租约机制后，发布检查改为等待实际租约、已到期重试和临近首次交付，保留未来重试及其时间。增加 300 秒优雅退出期限；未改业务队列、配置或任务状态，未强制终止登录。[保护调整记录 38576530587](https://github.com/tjt740/adobe-relay/actions/runs/38576530587) 成功。

## 上线核验与备份

- 两站 HTTPS、应用健康、实时事件流、报表策略、告警心跳均正常；两条告警规则、两套专属及全局 SMTP、联动配置和其他容器均保留。Sub2 环境与 Okad compose 只改应用镜像引用。
- 22:01:39 只读核验取得 946 条 Sub2 库存与 961 个 Okad 外部子号。异常筛选 429 个、401 为 143 个、408 为 344 个，其他代码、待验证和成功证据筛选均与库存逐项一致，分页无重复/遗漏。同一账号可同时有多个模型错误，因此错误码数量不能直接相加。这些是当时快照，不代表当前数量或全部账号已恢复。
- 上线资源哈希与发布代码一致。核验没有主动检测、登录、推送 Cookie、请求 Adobe 或发送邮件；原有后台业务继续运行。
- Sub2 切换备份：`/opt/adobe-relay/backups/20261014T135235Z-a2b46f5eac2bd04ddf6dc2d08843ec4843a086ad`。Okad 的 `okad-pre-release/` 保存准备期及实际切换时两份一致性 SQLite 备份、原配置和宿主文件；回退镜像基于当时实际容器，包含原筛选及运行叠加。
- 946 个 Sub2 账号、961 个 Okad 外部子号的 ID 均保留。55 个代理变化全部对应原自动分配/故障切换事件；Okad 的 Cookie、Token、邮箱凭据和登录状态未变化，3 个账号仅由原持久交付更新 `sub2_links.synced_at` 及更新时间。不会因正常业务变化恢复旧数据库或旧代理。
- Sub2 #446 在取得代理后恢复调度，#326/#401/#402 曾暂时等待代理，22:04 补查均已由原分配器重新分配并恢复；人工暂停字段未改。#179/#369 只有短期 `access_token`/`expires_at` 变化，原 Cookie 和其他凭据不变；补查时原联动验证时间为 22:03:45/22:03:42、结果为 `verified`，账号原检测失败和暂停仍保留。未取得单独的后台 Token 刷新日志，不把这些变化描述为本次主动验证或账号完全恢复。[来源核验 38577590618](https://github.com/tjt740/adobe-relay/actions/runs/38577590618) 成功，记录在 `business-state-source.json`、`business-state-source-v2.json`。
- 服务器保存 `before.json`、`after-sub2.json`、`after.json`、`okad-prepared.json`、`okad-deployed.json`、`runtime-verification.json`、`health-filter-verification.json`、`business-verification.json`、`account-state-audit.json`、`proxy-audit.json`、`okad-data-verification.json` 和来源补充审计。双项目 `check-rollback` 已通过，没有执行回滚。
