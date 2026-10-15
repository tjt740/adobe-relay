# Adobe 图片下载出口回退

标识：`adobe-download-fallback-20261015`。用户要求修复请求 `9f36c3f6-4da7-4895-b39d-f31a6f4dc336` 一类的 502，并授权测试后推送部署主服务器，保留现有功能和配置。

## 原因与行为

该请求已获得上游成品 URL，在账号 #374 的代理 #904 下载 3 次、183 秒后超时。截图中 9 条 502 均为下载失败，涉及 5 个账号、4 个代理。只读日志与无凭据 HEAD 对照复现了 #904 到 Adobe S3 存储域名的间歇 TLS 故障；同一代理访问 Adobe API 可达，服务器直连图片域名可正常建立 TLS。

原生图片仍先用账号原出口下载；仅可重试下载错误（网络、TLS、超时、408/429/5xx）在第二次尝试切到服务器直连，第三次如有需要继续直连。总次数仍为 3，每次仍按原 60 秒上限，受原上下文总截止时间限制。直连显式忽略环境代理，避免重新进入坏出口；只有带代理的默认下载客户端自动启用。自定义下载传输必须显式传入回退传输。

所有尝试 GET 同一个原始签名 URL，仅携带下载所需的 Accept 头，不携带 Adobe Cookie/Token；不换账号、不重新提交生成、不增加生成任务。失败仍返回原有错误语义。永久 4xx、内容拒绝、下载体积超限、安全策略拒绝及取消不触发出口回退。保留 HTTPS、证书验证、公网 IP 拨号校验、逐跳重定向校验、重定向次数和下载大小限制。

此改动仅作用于图片成品下载，视频沿用原下载策略。账号代理绑定、登录、上传、提交、轮询、Cookie 恢复、计费、返回字段、URL/Base64、metadata、模拟 usage、451、Okad 联动和邮件告警配置保持。内部成功耗时及失败详情增加 `download_fallback`，不增加客户响应字段。

## 验证

本地覆盖真实 CONNECT 隧道返回非 TLS 数据后直连取回原图、同 URL/同任务、重试上限、终态/取消/SSRF、环境代理隔离、24 路并发隔离、视频保持、多图张数/计费/URL/Base64/usage 保持。相关 Adobe 客户端、service 和 handler 执行 race 回归。正式发布流程继续执行既有后端、数据库、SMTP、前端及镜像构建检查。

仅发布 `173.234.15.150` / `https://creative.cheap`，不发布 Okad，不增加数据库迁移或配置项。发布前保存配置摘要、设置、规则和实际镜像，正式脚本保留备份；切换后核对保护项和实际业务变化，不恢复旧业务数据。线上回退效果优先读取自然请求的内部耗时及成功记录，核验不额外生成图片或发邮件。

## 回滚

审计目录 `/opt/adobe-relay/releases/adobe-download-fallback-20261015/`，回退镜像 `adobe-relay:before-adobe-download-fallback-20261015` 指向发布前实际应用 `5bfea0b97224f2a3dcdf976b2a6857f3ee176a10`。

```sh
python3 /opt/adobe-relay/releases/adobe-download-fallback-20261015/adobe-download-fallback-release.py check-rollback
```

改为 `rollback` 才执行代码回滚，恢复原同出口重试；保留当前业务数据、凭据、代理、SMTP、规则和 Okad。脚本依赖同目录 `image-response-release.py`，有后续应用或配置变化时拒绝覆盖，不绕过校验。

## 发布结果与实际核验

- 功能 [PR #57](https://github.com/tjt740/adobe-relay/pull/57) 已合并，实际应用 `0667d16cec9099c4d022089ef396acb62d0143e2`。[正式工作流 38696455670](https://github.com/tjt740/adobe-relay/actions/runs/38696455670) 的后端 race、数据库/SMTP、前端、构建及部署全部成功；北京时间 2026-10-15 15:17:36 启动新容器。本地 Adobe 全包及图片/service/handler/security race、4 项回滚保护通过。
- 15:22:36 的只读日志核验找到 **15 条自然请求在回退出口后成功取回图片**，均为 `submit_attempts=1`、`download_attempts=2`、`download_fallback=true`，每条恰好关联 1 条图片数量为 1 的原计费记录。没有重新生成或重复记账；这些是自然业务，核验未提交出图。计费按现有 `client:` 请求幂等键关联，不能把内部 `generation_request_id` 当成全部用量记录的主键。
- 两站 HTTPS、实时状态流、报表低频通知策略与新应用启动后的告警心跳正常。配置、环境文件（除镜像引用）、SMTP、告警规则、Okad 设置及其他容器身份保持。15:19 报表仍有 457 个待验证账号，不代表全部账号健康。
- 实际切换备份 `/opt/adobe-relay/backups/20261015T071724Z-0667d16cec9099c4d022089ef396acb62d0143e2` 与切换后 951 个账号 ID 一致。12 个代理变化全部由原自动切换/分配事件解释，9 个相应状态/调度变化随后读到 active/schedulable 且已分配代理。#177/#198/#254/#255 仅 access_token/expires_at 变化，Cookie 不变；未保留逐次成功刷新日志，不断言具体刷新来源，原凭据路径未改，不恢复旧业务数据。
- 发布标签 `release/adobe-download-fallback-20261015` 指向实际应用；前主干标签 `rollback/adobe-download-fallback-20261015-before-main` 指向 `34d8704a6ec7ecaef9feedb44dda05cf69656bf5`。`check-rollback` 已通过。
- 审计保存 `before.json`、`after.json`、`runtime-verification.json`、`business-verification.json`、`business-change-review.json`、`proxy-audit.json`、`account-state-audit.json`、`download-runtime-20261015T072236Z.json` 及代码回滚脚本。最初的下载核验尚未适配 console 日志、幂等键，原结果保留；以 07:22:36 UTC 的修正核验为准。辅助核验脚本的修正没有修改应用或业务数据。

## 独立的既有内存压力

合并前已确认旧版容器触及现有 **2 GiB** cgroup 内存上限，14:10–15:05 的内核日志含 17 次 Sub2 OOM 终止。新版本在 15:19:47、15:23:41 也发生同类 OOM，15:24 核验已自动重启并健康。本次保留内存/并发配置，未解决该独立问题；它仍可能中断其他请求，不能把下载回退成功等同于服务整体稳定。证据在 `preexisting-memory-pressure.json`、`postflight-memory.json`，不要将已在发布前发生的 OOM 误归因为本次修复。

后续归档只改说明和只读核验脚本，使用 `[skip ci]`；实际线上应用仍以上述 SHA 为准。
