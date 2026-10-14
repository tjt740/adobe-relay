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

开发阶段相关 Go 测试与 race、隔离 PostgreSQL 证据测试通过；原生产脱敏回放因没有提供快照而跳过。Okad 合并运行筛选后 94 项后端回归及错误状态/原筛选两套浏览器回归通过。部署脚本保护测试通过。正式流程及上线核验结果完成后归档；此段不提前宣称已发布。
