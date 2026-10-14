# Adobe 图片 URL 与真实用量响应

- 迭代：`adobe-image-url-usage-20261014`，仅发布主服务器 `173.234.15.150` / `https://creative.cheap`。
- 用户授权合并 main、推送部署，保留功能、配置和数据；无法取得真实 token 时返回 `null`，不估算。
- 发布前 main：`39cfa1170998e9bfd97cf09c275f221e79a0a644`；实际线上应用：`6890f439ac457a938e10892e3b94c274d0b183aa`。文档提交与应用镜像版本分别记录。
- Okad 当前实际版本为 `ef022152a597f1c2b38f9657168c632bad070d85`，本次不替换其镜像或容器。
- [PR #46](https://github.com/tjt740/adobe-relay/pull/46) 已合并，实际应用为 `020430320d0fbc31258ea21fcdfdd08d10101697`。[正式工作流 38563759181](https://github.com/tjt740/adobe-relay/actions/runs/38563759181) 成功，北京时间 2026-10-14 **20:38:48** 切换，公网 HTTPS 检查通过。

## 结果与兼容

显式 `response_format: "url"` 优先原对象存储，再使用本次下载验证过且交付字节未改变的 Adobe HTTPS 临时链接；不满足条件时保留原 Base64 交付。默认格式与显式 Base64、图片编辑、多图顺序、计费、调度、账号恢复和其他协议路径保持。增加实际输出尺寸/格式等元数据；多图无法统一的元数据不虚报。

两次真实 `gpt-image-2` 生成及完成后的持续观察没有取得 token 用量，因此实际此类响应为 `usage: null`。只兼容明确、合法的上游标准 `usage`，不根据字符、图片、额度或价目估算。全部图片有可信总量才汇总。详情及脱敏证据见 [接口说明](../../docs/ADOBE_IMAGE_RESPONSE_USAGE.md)。无数据库迁移，不增加正常生成请求数。

## 验证

- 本地服务、网关、路由相关 `-race` 检查 368 项通过；客户端全包 445 项通过，3 项既有 HTTP 回环下载测试失败，已在原 main 重现，未放宽生产下载校验。
- 发布脚本 10 项检查通过，包括正式切换失败恢复和本次回滚版本/配置保护。
- 正式主服务器流程的后端 race/数据库/SMTP、337 项前端回归、部署脚本检查和镜像构建通过。
- 上线后两站 HTTPS、实时状态流、报表策略及新容器启动后的告警心跳通过。两条 Adobe 专属告警规则、全部 Sub2 设置（含两套私有 SMTP 和全局邮件）、Okad 配置/容器、其他服务容器及 Nginx 均未改变；Sub2 环境只改变镜像引用。发布核验未发送邮件或主动请求 Adobe。

## 实际业务数据核对

与实际切换前的数据库备份比对，946 个账号 ID 全部保留，凭据无变化。20:42:59 的同一快照内，27 个代理变化均能由原有 `proxy_failover_events` 串联解释；#722、#728、#730 的状态/调度变化对应原代理分配器暂时没有可用代理时的自动暂停。计费、代理分配器和数据库迁移文件与发布前完全一致，未恢复旧绑定或旧状态。

首次核验恰好观察到 #301、#323、#340 从代理 790 暂时解绑。旧审计辅助程序把事件中的 `0` 和数据库中的 `NULL` 当成不同代理，且分开查询账号与事件，因而拒绝通过。此次只修正只读核验程序：统一无代理表示，并在同一个数据库语句快照中读取账号与事件。原始失败核验保留在 `initial-*`，正式核验和状态来源分别保存在 `business-verification.json`、`business-state-source.json`，没有因此修改应用或业务数据。

## 备份、审计与回滚

审计目录 `/opt/adobe-relay/releases/adobe-image-url-usage-20261014/`。`image-response-release.py before` 在发布锁下保存配置摘要、两条告警规则、全部设置摘要、账号摘要和各服务容器身份，并将实际线上镜像保留为 `adobe-relay:before-adobe-image-url-usage-20261014`。正式 `release.sh` 在实际切换前另行备份数据库、环境与配置、应用数据。

上线后核对 `after.json`、`runtime-verification.json`、实际切换数据库备份与业务变化来源。回滚仅恢复旧应用，沿用当前业务数据、Cookie、代理、SMTP、告警策略和 Okad；不恢复旧数据库。先运行只读检查：

```sh
python3 /opt/adobe-relay/releases/adobe-image-url-usage-20261014/image-response-release.py check-rollback
```

需要撤回时将末尾改为 `rollback`。脚本要求当前应用与 `expected-sha` 一致，回退镜像身份正确，配置/设置/告警规则和其他服务未发生后续变化，否则拒绝覆盖；不能绕过保护。正式切换仍原子核对应用版本。现有所有历史回滚镜像继续保留。

实际备份为 `/opt/adobe-relay/backups/20261014T123845Z-020430320d0fbc31258ea21fcdfdd08d10101697`。回退镜像对应此前实际运行的 `6890f439`，回滚保护只读检查已通过，回执为 `rollback-check.json`。当前应用镜像 ID 为 `sha256:4528ea84b63209c20253b965669cc7c68a05cd59543f2acb364dbe5aa1332866`。

发布前主干标签为 `rollback/adobe-image-url-usage-20261014-before-main`，发布标签为 `release/adobe-image-url-usage-20261014`，已指向实际应用提交。后续仅审计说明归档使用 `[skip ci]`，避免重复部署。
