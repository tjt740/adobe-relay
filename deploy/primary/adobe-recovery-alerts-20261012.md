# Adobe 自动恢复保护与批量异常告警

迭代标识：`adobe-recovery-alerts-20261012`。仅发布到主服务器 `173.234.15.150`。

## 行为

- Adobe 401 按额度、权益、鉴权分类。额度/权益不足不重新登录；鉴权失败先废弃被拒绝的 access token，使用原 Cookie 换取 Token，只有确认 Cookie 失效才进入自动恢复。
- 408 保持临时上游故障语义，不触发重新登录。检测接口保留原始错误类型，避免仅凭显示文字或 HTTP 401 触发恢复。
- Okad 对同一邮箱的所有自动登录入口共用持久化互斥与冷却；失败后至少 5 分钟，成功后至少 15 分钟。重叠请求复用已完成结果。手动登录不受自动冷却限制，但仍受并发互斥保护。
- Sub2 每轮故障最多实际尝试登录两次；Okad 明确返回“冷却中，未执行登录”时，不消耗登录次数，并持久化下一次允许时间。

## 告警设置

在 Sub2 运维监控的告警规则中配置：

| 字段 | 值 |
|---|---|
| 名称 | Adobe 401/408 批量账号异常 |
| metric_type | adobe_401_408_accounts |
| 条件 | >= 5 |
| 时间窗口 | 最近 10 分钟 |
| sustained_minutes | 1（首次满足即告警） |
| cooldown_minutes | 30 |
| severity | P1 |
| filters | `{"platform":"adobe","email_recipient":"tjt740@gmail.com"}` |
| notify_email / enabled | true |

指标按约 2 分钟频率评估，复用运维评估器，不改变其他规则的评估频率。仅读取业务错误日志，不请求 Adobe、不测试账号、不更新 Cookie。401/408 合并按账号去重；包括换号后最终请求成功前的失败尝试，并按每次尝试的账号归属统计。

邮件包含北京时间窗口、受影响账号总数、分别出现 401/408 的账号数量和错误条数、401 原因分类、最多 100 个账号明细。一次持续异常成功发送一封；恢复后新一轮异常仍受 30 分钟冷却限制。发送失败的持续异常会重试，已发送状态保存在数据库，重启不重复发送。新规则收件人独立配置，不启用其他规则的全局收件人。

本次使用独立告警发信配置，存于私有设置项 `ops_adobe_alert_mail_private`，内容为 recipient 和 smtp（SMTPConfig）对象。仅 Adobe 401/408 指标可以调用此通道，且只能发给其指定收件人。全局 SMTP、注册验证、其他通知模板和收件人保持原状。Gmail 使用 smtp.gmail.com / 465 / TLS 和应用专用密码；凭据不进入规则 filters、API 响应或代码仓库。

## 验证与发布

已用隔离 PostgreSQL、SQLite 和测试 SMTP 覆盖：并发互斥、冷却持久化、两次尝试预算、401 分类、408 不登录、历史数据排除、换号成功后的错误归属、账号去重、邮件失败重试和重启去重。Okad 完整后端测试 447 项通过。线上仅执行只读统计和查询计划检查，不进行真实 Adobe 登录压测。

Sub2 使用正常 main 发布流程。Okad 基于发布时的实际运行容器制作补丁镜像，仅替换本次六个后端源码文件；保留服务器监控和移动端等已上线叠加功能。发布前记录源码哈希、容器镜像和配置摘要，发布后确认变更范围。

## 回滚

本次发布前另存两个镜像：

- `adobe-relay:before-adobe-recovery-alerts-20261012`
- `adobe-team-cookie:before-adobe-recovery-alerts-20261012`

执行已有发布脚本恢复对应镜像，保留当前业务数据：

```sh
bash /opt/adobe-relay/deploy/primary/release.sh adobe-relay:before-adobe-recovery-alerts-20261012
bash /opt/adobe-relay/deploy/primary/okad-release.sh adobe-team-cookie:before-adobe-recovery-alerts-20261012
```

回滚前先关闭 `adobe_401_408_accounts` 规则，旧版本不认识该指标。迁移 246 只放宽恢复次数下限至 0，Okad 只增加 RecoveryJob.automatic 字段；旧代码兼容保留的结构，不恢复旧数据库。发信配置无需删除。服务器审计记录放在 `/opt/adobe-relay/releases/adobe-recovery-alerts-20261012/`。如后续有新发布，先核对差异，避免覆盖后续功能。
