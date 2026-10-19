# Okad 自定义分组双端发布（2026-10-20）

标识 `okad-custom-groups-20261020`。仅发布主服务器 `173.234.15.150`，入口为 `https://creative.cheap/okad/external-members` 与 `https://creative.cheap/admin/accounts`。

## 行为与版本

外部子号导入时可添加和选择自定义分组，已有账号支持批量改组。公司、越南、非公司账号类型与分组组合筛选；列表、导出和筛选批量操作使用相同范围。Sub2 增加「Okad 创建的分组」筛选，选项来自已同步账号，独立于 Sub2 业务分组。改组只更新身份核对后的元数据，不触发登录、Cookie 推送或出图检测。

- Sub2 实际应用／镜像：`9baadf2d1b525c318f9a7be0d6f2bf37381d2fb2`；功能 PR [#83](https://github.com/tjt740/adobe-relay/pull/83)，功能合并 `976a4328b252999026a76d788fc2744f42fc71c3`。
- Okad 实际应用／镜像：`879e9ded87256e678e41579f04755cabcf610728`；功能 PR [#13](https://github.com/tjt740/adobe-team-cookie/pull/13)，功能合并 `1c1344ed18bb7974f4de8e3c33ed31f4024e9e43`。以实际运行的 1fded29 为基础增量更新 11 个应用文件，保留已上线的越南登录、工作区、筛选与监控功能。
- [正式检查和 amd64 镜像构建](https://github.com/tjt740/adobe-relay/actions/runs/39304456235) 成功，`build_only=true`。后续运维和归档提交不改变应用源码，镜像核验使用上面的实际应用 SHA。

## 切换与核验

北京时间 **03:00:29** 临时 Sub2 实例接收新连接，旧实例排空后切回正式实例，**03:02:24** 临时实例、NAT 与滚动租约清理完成。Okad 先用断网的 SQLite 备份副本演练增量迁移，**03:02:56** 完成健康切换。

正式后端、数据库、SMTP、前端及构建通过；Okad 相关后端 75 项、桌面／手机及根路径／`/okad` 浏览器回归通过。线上使用独立的无 Cookie、不可调度临时记录，验证分组创建、重名校验、改组后旧组移出／新组命中、未分组和越南类型组合。真实浏览器在 1440px 与 390px 完成创建、批量改组及双端组合筛选，未模拟接口响应，JavaScript 错误为 0。测试数据已清理，短时验证会话文件已移除；未主动登录 Adobe、生成图片或发送测试邮件。

**03:10:48** 最终检查通过：两站 HTTPS、实时事件流、正式进程告警心跳正常。两分钟 13 次被动采样均健康，0 OOM／OOM kill／重启；这不是高峰负载或 Adobe 账号恢复证明。

## 配置和数据保留

保留 Sub2 **54 GiB／Go 43008 MiB／GOMAXPROCS=16／CPU shares 4096**、Okad **3 GiB** 及全部其他服务。持久配置仅更新两端镜像引用；网络、SMTP、告警策略、密钥及用户设置摘要保持。

977 个原有 Sub2 账号 ID、凭据、状态、调度与人工暂停保持，59 个代理变化均匹配原自动切换事件链；未恢复旧绑定。Okad 原有 1063 个外部账号及 users、settings、adobe_accounts、adobe_members、emails 的既有内容摘要保持。只新增分组表、可空 group_id 和索引，SQLite 完整性检查通过。临时验证分组及 Okad 账号已删除，Sub2 临时账号经正常删除接口清理，不再出现在有效库存中。

## 审计与回退

- 审计：`/opt/adobe-relay/releases/okad-custom-groups-20261020/`，含两端 before／after、分组接口、真实浏览器、数据保留、被动观察和最终 paired-verification 记录。
- PostgreSQL 切换备份：`/opt/adobe-relay/backups/20261019T190117Z-9baadf2d1b525c318f9a7be0d6f2bf37381d2fb2/`，转储可读取。Okad SQLite、配置与宿主源码备份位于审计目录 `okad/backup/`。
- 回退镜像保留：`adobe-relay:before-okad-custom-groups-20261020`（原 7dfc317）及 `adobe-team-cookie:before-okad-custom-groups-20261020`（保留原 1fded29 的实际运行内容）。
- 两仓库发布标签 `release/okad-custom-groups-20261020` 指向实际应用，`rollback/okad-custom-groups-20261020-before-main` 指向各自发布前主干。
- Sub2 回退保护在 Okad 切换前核对通过；Okad 回退保护与两端配套保留检查在最终核验通过。真正回退须先核对后续两端版本、配置及活动任务，并为 Sub2 滚动排空；只恢复代码，保留当前数据库和用户操作，勿绕过旧版本保护或恢复旧数据库。

本次修复同时按北京时间日期更新网站每日更新日志。后续仅归档说明和验收脚本，使用 `[skip ci]`，不重复部署。
