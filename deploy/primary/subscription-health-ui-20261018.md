# 订阅界面与每日更新日志

标识 `subscription-health-ui-20261018`。仅发布主服务器 `173.234.15.150` / `https://creative.cheap`。

## 行为

订阅链接输入框从两行提高到默认六行、最小 10rem，保留纵向拖动与链接显隐；健康、部分可用和不可用分别采用绿、橙、红淡色印章水印。正文使用中性色，保留实际节点数量、失败原因、待检测、记录过期与订阅更新失败等区别。没有修改健康判断或订阅操作逻辑。

网站更新日志按北京时间实际发布日期倒序汇总，同一天多次发布合并，最新日期默认展开。补齐 10 月 12–18 日已有发布摘要，保留原专题、场景说明和文档阅读/下载。用户要求此后每次更新和修复均随版本同步维护当天日志；已写入仓库 AGENTS.md 及项目根约定。未发布或仅测试的功能不能写成已上线。

本次只改前端及仓库约定，后端源码与原应用 `55434e9055871d23d62281352831e7d4d2600d97` 相同，无迁移或新增配置。

## 发布

- 实际应用与镜像提交 `7dfc317773c88153e7b70a35c5b5207d32de519b`。
- 功能 PR [#81](https://github.com/tjt740/adobe-relay/pull/81)，合并 `3691caa19eb2915352713415a11ba5967b3a66cb`，使用 `[skip ci]` 避免默认重复部署。
- [正式检查及镜像构建](https://github.com/tjt740/adobe-relay/actions/runs/39135974046)，`build_only=true`。
- 本机 SSH 握手超时后，改由既有 GitHub 主服务器发布入口执行同一受保护滚动流程；[发布与核验](https://github.com/tjt740/adobe-relay/actions/runs/39137314414)。运行辅助分支 `codex/publish-subscription-ui-20261018`，不作为应用镜像 SHA，也未用其覆盖 main 的原审计工作流。
- 北京时间 **2026-10-18 22:47:10** 完成临时实例、NAT 和滚动租约清理。两次切换保留现有连接，按在途槽位和待上传连接等待稳定排空。
- 发布标签 `release/subscription-health-ui-20261018`；发布前主干标签 `rollback/subscription-health-ui-20261018-before-main` 指向 `0c4c4c68c0441987bffff3ddc54b73e7a762df00`。

## 核验与保留

本地 74 项订阅、健康面板、语言及每日归组检查、TypeScript、ESLint 和生产构建通过；正式后端/数据库/SMTP、489 项前端检查与 amd64 镜像流程通过。浏览器检查桌面、375px 窄屏、深色主题、英文、历史日期与原专题展开。生产 HTTPS 资源已核验六行输入框、水印 CSS、每日归组与当天条目。

两站 HTTPS、实时事件流、报表策略和临时实例清理后的正式进程告警心跳正常。两分钟 13 次自然采样通过，0 OOM / OOM kill、0 重启；不作为真实高峰或 Adobe 业务恢复证明。核验没有主动生成、检测、登录或发送邮件。

Sub2 **54 GiB / Go 43008 MiB / 图片逻辑预算 37.8 GiB / GOMAXPROCS=16 / CPU shares 4096**、Okad **3 GiB** 及其实际镜像、其他容器、持久配置、人工调度、SMTP 和告警均通过保护核验。与实际切换备份相比，原有 956 个账号，当前 956 个，缺失 0 个；人工暂停标记保持。凭据、状态、调度及人工暂停与实际切换备份一致；12 个代理绑定变化均有原自动切换事件链，具体记录保存在 business-verification.json；保留当前业务和分组 2 / 7 的用户操作，没有恢复旧数据。

## 审计与回退

审计目录 `/opt/adobe-relay/releases/subscription-health-ui-20261018/`。切换备份 `/opt/adobe-relay/backups/20261018T144600Z-7dfc317773c88153e7b70a35c5b5207d32de519b`，PostgreSQL 转储已验证可读取。镜像 ID `sha256:1828bce878c913dcdf89abf6b43b34b1b4278c04a9bcee4191c1a4e23534babf`。

保留回退镜像 `adobe-relay:before-subscription-health-ui-20261018`，对应实际旧应用 `55434e9055871d23d62281352831e7d4d2600d97`。只读条件检查 `python3 /opt/adobe-relay/releases/subscription-health-ui-20261018/audit.py check-rollback` 已通过。真正回退前重新核对后续版本、配置及在途任务，使用受控滚动排空后恢复代码；保留当前数据库、凭据、人工状态、SMTP 和 Okad，不绕过保护或恢复旧数据库。

后续归档仅说明并使用 `[skip ci]`，不重复部署，应用核验继续使用上面的实际应用 SHA。
