# 第二个 Adobe 专属告警邮箱（2026-10-14）

迭代 `adobe-second-alert-mail-20261014`，仅发布主服务器 `173.234.15.150` / `creative.cheap`。

原 `tjt740@gmail.com` 发信配置和规则 9 保持。为用户指定的 `lhaiming65@gmail.com` 增加独立 Gmail SMTP 和规则 10，发件名仍为「Sub2告警」。新规则沿用 20% 且至少 5 个账号、10 分钟窗口、持续 4 分钟确认、60 分钟冷却和成功证据恢复条件。每条规则有独立事件回执，不会因为第二个邮箱失败而重复发送第一个邮箱的邮件。

新增发信配置保存在原私有设置 `ops_adobe_alert_mail_private.additional`，不进入全局 SMTP、规则 filters、源码或日志。每条 Adobe 规则显式指定单个收件人，精确选择对应 SMTP；不群发、不将原邮箱凭据用于新邮箱。未匹配或重复配置拒绝发信；原私有 JSON 和原人工报表默认收件人兼容。其他类型邮件继续走原通道。

只修改发信配置选择，不修改账号、Cookie、代理、调度或 Adobe 检测，无数据库迁移，不发布 Okad。新邮箱经 SMTP 认证验证；上线后按用户明确请求发送一次真实线上报表，保留手机排版与完整 HTML 附件，独立回执防止重发。人工报表不会修改自动告警状态或冷却。

验证覆盖两台独立测试 SMTP、原邮箱兼容、收件人拒绝/歧义处理、无全局 SMTP 污染、发送回执及重启去重；PostgreSQL 覆盖两条规则的独立发送和冷却。正式 CI 继续执行原后端、前端和构建回归。

服务器审计目录 `/opt/adobe-relay/releases/adobe-second-alert-mail-20261014/` 保存实际应用 `expected-sha`、配置摘要、规则快照、发送回执和上线核验；不记录授权码。回退镜像 `adobe-relay:before-adobe-second-alert-mail-20261014` 为此前实际应用 `58638a1b`。两仓库之间本次只发布 Sub2。

只读回滚校验：

```sh
python3 /opt/adobe-relay/releases/adobe-second-alert-mail-20261014/release.py check-rollback
```

撤回本次代码及新规则：

```sh
python3 /opt/adobe-relay/releases/adobe-second-alert-mail-20261014/release.py rollback
```

先检查后续版本/配置；脚本拒绝覆盖后续变化。回滚仅停用新规则并恢复 Sub2 代码，保留当前数据库、账号、原邮件和所有私有发信配置，不重复发送任何邮件。新规则和配置均可供后续重新启用。
