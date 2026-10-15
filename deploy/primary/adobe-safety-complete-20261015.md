# 图片违规 451 完整覆盖与调用说明

标识：`adobe-safety-complete-20261015`。用户要求补齐 Adobe 直连、本站审核、第三方中转的违规处理，加入回归及完整客户示例，并核查客户 usage 数字来源。

## 改动

- Adobe 显式安全错误优先于 HTTP 成功、任务状态和 outputs 占位/链接解析；增加参考图、输入媒体及明确提示词策略错误码。鉴权、权益、额度、地域限制和普通参数错误不按提示词违规推断。
- 图片入口的本地审核拦截统一 451，保留原错误信封、文案和审计记录；不修改审核决策、非图片入口、审核故障 503 或原配置。Gemini 的 451 映射为 PERMISSION_DENIED。
- OpenAI 兼容图片转发对明确的内容策略错误统一 451，在凭据恢复、冷却和换号判断之前识别。非流式 JSON、HTTP 错误、SSE 错误/未完成内容过滤和已有文本拒绝识别均覆盖；错误正文不做新增模糊匹配。正常图片、usage、计费和部分图片计费保持。
- 原始中转 SSE 按帧解析后保留正常帧；首个错误可返回 HTTP 451。流已提交 200 后不能改写 HTTP，错误 JSON/SSE 增加 error.status_code=451；保留已有流和 JSON 心跳功能。
- 发布流程增加 Adobe 客户端完整 unit/race，以及图片 handler/service、审核、异步/批量、流式和 Google 信封回归。
- [客户调用说明](../../docs/ADOBE_IMAGE_CLIENT_GUIDE.md) 包含 curl、Python、图生图、URL/Base64、临时链接、usage=null、451 和流式边界。
- [usage 来源分析](../../docs/ADOBE_IMAGE_USAGE_PROVENANCE.md)：3336 可由官方公式独立复算；2120 来源无法证实，5456 为相加。未新增估算返参或修改真实 usage/null 规则。

## 发布与保护

仅发布 Sub2 到主服务器 173.234.15.150。预检时应用为 `ecb3e918ec143046b1cdf65181a9e926e322030d`，950 个未删除账号；已核对其他任务及三把发布锁。保留现有配置、数据、SMTP、告警、凭据、代理、Okad 实际容器及已启用联动。无数据库迁移，本轮不发起真实 Adobe 出图、登录或邮件。

发布前主干为 `acd5a2f9bfaebd1f6b19e973fedd14e0090ce214`。功能 [PR #52](https://github.com/tjt740/adobe-relay/pull/52) 合并后的实际应用为 `9ff382000550d0c9010df5c2d5b90c18c950310b`；[正式工作流 38675033074](https://github.com/tjt740/adobe-relay/actions/runs/38675033074) 的后端、前端、构建及发布全部成功。主服务器于北京时间 2026-10-15 11:22:44 启动新 Sub2 容器。

发布标签 `release/adobe-safety-complete-20261015` 指向实际应用；`rollback/adobe-safety-complete-20261015-before-main` 指向上述发布前主干。审计目录 `/opt/adobe-relay/releases/adobe-safety-complete-20261015/`。切换备份 `/opt/adobe-relay/backups/20261015T032241Z-9ff382000550d0c9010df5c2d5b90c18c950310b`。

保留前版本镜像 `adobe-relay:before-adobe-safety-complete-20261015`。仅检查：

```sh
python3 /opt/adobe-relay/releases/adobe-safety-complete-20261015/adobe-safety-complete-release.py check-rollback
```

将末尾改为 `rollback` 才执行代码回退。脚本依赖同目录 `image-response-release.py`，核对应用版本、镜像内容、配置、设置和其他容器，调用正式 release.sh 切换应用；不恢复旧数据库，不绕过后续变更检查。

## 本地验证

带 race 的客户端、服务、图片 handler、Google 信封相关回归通过；新增构造响应覆盖 200/201/202/400/401/403/451/500、outputs 占位/链接、参考图拒绝、无凭据恢复/重复调用、OpenAI/Gemini 错误信封、流开始前/后、多行 SSE、普通错误及审核故障保留。客户 JSON/Python/bash 示例通过语法检查，回滚保护测试通过。

Adobe 客户端全包在清除当前终端 HTTP(S)/ALL_PROXY 环境变量后通过。之前三项本机下载测试失败来自测试进程继承环境代理后启用公网 DNS 防护；此次未修改下载安全策略或测试夹具。

## 上线核验与业务变化归因

- 两站 HTTPS、实时状态事件、报表策略及新容器启动后的告警心跳正常。配置、环境文件（除镜像引用）、Sub2 设置（含 SMTP）、告警规则、Okad 设置及其他容器身份与重启次数均保持。没有 Nginx 错误拦截规则覆盖应用的 451。
- 与实际切换备份比较，950 个账号 ID 全部保留；没有新增或删除。9 个代理变化全部有原代理分配/故障切换事件链，未恢复旧绑定。
- #853、#1115 在备份中均为等待分配代理且 `proxy_auto_paused=true`；原代理分配任务于 11:23:15 分配后恢复调度。对应原代码 `backend/internal/proxyfailover/allocation.go` 本次未变，不能将此变动解释为部署覆盖或新 Cookie 成功证据。
- #9、#14 只更新原 Token 刷新字段 `_token_version`、`access_token`、`expires_at`；11:22:48 原刷新任务日志与之对应。Cookie 未变，人工暂停及不参与调度状态保持；原 `token_refresh_service.go` 本次未变。
- 11:23 核验报表有 463 个待验证账号；部署健康不代表全部 Adobe 账号恢复。核验未发邮件、未主动生成图片、未触发登录。
- 证据包括 `before.json`、`after.json`、`runtime-verification.json`、`business-verification.json`、`proxy-audit.json`、`account-state-audit.json`、`business-change-review.json`。回滚检查已通过；回退镜像对应发布前实际 `ecb3e918`，保留当前业务数据。
- 后续客户示例及发布归档提交仅更新文档，使用 `[skip ci]`，不重复构建或重启；线上应用版本应以上述实际 SHA 为准。
