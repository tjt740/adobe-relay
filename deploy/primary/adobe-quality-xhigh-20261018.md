# Adobe xhigh 响应名称修复

标识：`adobe-quality-xhigh-20261018`。仅部署主服务器 `173.234.15.150` / `https://creative.cheap`。

## 结果

Adobe Cookie 直连 GPT Image 2.5 的生图和编辑请求传入 `quality=xhigh` 时，响应现在保留 `quality=xhigh`；`max` 继续返回 `max`。按用户选择，两者提交给 Adobe 的值仍同为 `detailLevel=7`，本次只修正响应标签。2 / 1.5 的模型上限、默认值及其他模型的省略行为保持。

- 实际应用及镜像提交：`71e43c345e010d56909f59f7f851cf073840be1b`。
- 功能 PR：[77](https://github.com/tjt740/adobe-relay/pull/77)，合并提交 `4cf2afd6281385d43c2691306e9aab68a8db55d6`。
- 正式检查及 amd64 镜像构建：[39091134538](https://github.com/tjt740/adobe-relay/actions/runs/39091134538)，全部通过。以 `build_only=true` 生成镜像，随后使用正式发布脚本受控滚动；合并信息带 `[skip ci]`，避免默认工作流重复重启正式实例。
- 发布标签：`release/adobe-quality-xhigh-20261018`；发布前主干标签：`rollback/adobe-quality-xhigh-20261018-before-main`，指向 `96b84b2583fed32664d7a6372f4e4d84529410a4`。该主干代码树与发布前实际应用 `ef0a02590caff30f02ae1019d743c03c6676f3ba` 相同。

北京时间 2026-10-18 **11:02:22** 新连接转入新版临时实例，旧实例稳定排空 45 秒后，**11:03:28** 正式实例接回新连接；**11:04:17** 临时实例、临时 NAT 和滚动租约清理完成。两次排空均同时检查请求槽位和待上传连接，Nginx 配置文件保持。

## 保留与验证

应用行为仅调整响应质量标签，未改提交参数、图片交付、计费或数据库迁移。保留代理可靠性及更新记录页面等已上线功能。

- 本地回归覆盖 Flare、Sunburst、Prism、大小写和空格、旧模型上限、模型映射、生图、编辑、URL、Base64、转存失败及磁盘缓冲，同时断言上游仍收到 `7`。正式后端、数据库、SMTP、前端及镜像检查通过。滚动切换 6 项保护测试和回滚守卫 6 种拒绝场景通过。
- Sub2 容器 **54 GiB**、Go **43008 MiB**、图片逻辑预算 **37.8 GiB**、`GOMAXPROCS=16`、`cpu_shares=4096` 保留，无 CPU 硬配额。Okad **3 GiB**，其他服务资源及容器保持。
- 原持久配置、除镜像引用外的环境、Nginx、全局和专属 SMTP、告警规则及 Okad 配置摘要一致。Okad 实际镜像 `adobe-team-cookie:external-timeout-20261016T193417Z`、容器和已有功能保持。
- 与实际切换备份比对，**956 个账号**的 ID、凭据、状态、调度及人工暂停一致。15 个账号的代理绑定变化均有原自动切换事件链，保留实时业务变化，未恢复旧绑定或旧数据库；分组 2 / 7 继续保留用户当前操作。
- 两站 HTTPS、实时事件流、报表策略及新进程告警心跳通过。本次没有主动请求 Adobe 生成、检测、登录或发送测试邮件。
- 11:04:41–11:06:41 两分钟 13 次自然采样均健康，0 OOM / OOM kill、0 重启，内存采样峰值约 85.42 MiB。11:06:48 最终核验通过，正式进程在临时实例清理后的告警心跳为 11:06:23。该观察不作为真实高峰或 Adobe 吞吐证明。

## 备份与回退

- 服务器审计：`/opt/adobe-relay/releases/adobe-quality-xhigh-20261018/`，含 before/after、发布元数据、切流及排空记录、业务数据核验和运行核验。
- 实际切换备份：`/opt/adobe-relay/backups/20261018T030310Z-71e43c345e010d56909f59f7f851cf073840be1b`，PostgreSQL 转储已验证可读取。
- 回退镜像：`adobe-relay:before-adobe-quality-xhigh-20261018`，保留发布前实际应用 `ef0a02590caff30f02ae1019d743c03c6676f3ba`。
- 只读条件检查：`python3 /opt/adobe-relay/releases/adobe-quality-xhigh-20261018/audit.py check-rollback`。存在后续版本、资源或配置变化时拒绝覆盖。
- 真正回退前重新审查当前版本和配置，按受控滚动流程排空在途请求，再使用正式发布脚本恢复已保留镜像；保留当前业务数据、SMTP、人工调度及 Okad，不绕过保护或直接恢复旧数据库。

后续发布归档只改说明并使用 `[skip ci]`；实际应用 SHA 仍为 `71e43c345e010d56909f59f7f851cf073840be1b`。
