# Adobe 模型列表兜底修复

标识：`adobe-models-fallback-20261018`。仅发布主服务器 `173.234.15.150` / `https://creative.cheap`。

## 结果

Adobe 分组没有可调度账号且模型白名单未启用时，`/v1/models` 和 `/models` 原先错误回退到 13 个 Claude 模型。现在返回已有的 16 个 Adobe 图片模型；账号映射和启用的白名单仍按原规则筛选。仅修改模型发现的兜底分支，不修改账号状态、人工暂停或调度，不新增数据库迁移。模型目录不表示当前账号有能力完成真实生成。

- 实际应用及镜像提交：`55434e9055871d23d62281352831e7d4d2600d97`。
- 功能 PR：[79](https://github.com/tjt740/adobe-relay/pull/79)，合并提交 `b4d6e1c9e80ea0f1d88f4ea4597983a7410776e4`。合并带 `[skip ci]`，避免自动重复部署。
- 正式检查及 amd64 镜像构建：[39122469168](https://github.com/tjt740/adobe-relay/actions/runs/39122469168)，使用 `build_only=true`，全部通过后受控滚动发布。
- 发布标签：`release/adobe-models-fallback-20261018`；发布前主干标签：`rollback/adobe-models-fallback-20261018-before-main`，指向 `2f6657b3b297b14101273b873d747721c22f664a`。

北京时间 2026-10-18 **19:34:02** 正式实例接回新连接，随后完成临时实例排空、临时 NAT 和滚动租约清理。两次切换均检查在途槽位及待上传连接并等待稳定排空，Nginx 配置文件保持。

## 核验与保留

使用已有密钥 ID 12（不保存密钥值）核验 Adobe 分组 #7：`/v1/models` 与 `/models` 均返回完整 16 个 Adobe 图片模型，`/v1/models/gpt-image-2` 返回 200，`/v1/models/claude-opus-4-6` 返回 404。密钥绑定、分组配置及白名单开关的指纹保持一致。本次不启用任何账号，不调用生成、检测、登录或发送测试邮件。

新增回归先在旧代码复现，修复后模型列表和 Codex 模型相关 race 回归通过。新增 8 个场景覆盖两个列表入口、空候选、关闭/开启白名单及已有账号映射；滚动切换 6 项保护测试、正式后端/数据库/SMTP 和 486 项前端检查通过。

保留 Sub2 **54 GiB**、Go **43008 MiB**、图片预算 **37.8 GiB**、`GOMAXPROCS=16` 和 `cpu_shares=4096`。Okad 保持 **3 GiB**、实际镜像 `adobe-team-cookie:ee6bdc2f18929e960f79305962c81563a912d491` 及其容器和配置；其他容器、全局/专属 SMTP、告警规则、持久配置均通过保护检查。

与实际切换备份相比，956 个账号的 ID、凭据、状态、调度和人工暂停保持。21 个代理绑定变化均有原自动切换事件链，保留实时业务数据及分组 2 / 7 的用户操作，不恢复旧代理或旧数据库。

北京时间 **19:35:37–19:37:37** 两分钟 13 次自然采样通过，0 OOM / OOM kill、0 重启；**19:37:38** 最终核验通过。两站 HTTPS、实时事件流、报表策略、正式进程告警心跳和清理状态均通过。该观察不作为真实高峰或上游账号恢复证明。

## 备份与回退

服务器审计：`/opt/adobe-relay/releases/adobe-models-fallback-20261018/`。实际切换备份：`/opt/adobe-relay/backups/20261018T113343Z-55434e9055871d23d62281352831e7d4d2600d97`，PostgreSQL 转储已验证可读取。

回退镜像 `adobe-relay:before-adobe-models-fallback-20261018` 为发布前实际应用 `71e43c345e010d56909f59f7f851cf073840be1b`。只读条件检查已通过：`python3 /opt/adobe-relay/releases/adobe-models-fallback-20261018/audit.py check-rollback`。真正回退前复核后续版本、配置与在途任务，使用受控滚动和正式发布脚本恢复应用镜像；保留当前业务数据、人工状态、SMTP 和 Okad，不绕过保护或恢复旧数据库。

后续归档仅修改说明并使用 `[skip ci]`，不重复部署，实际应用 SHA 仍为 `55434e9055871d23d62281352831e7d4d2600d97`。
