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

发布前主干为 `acd5a2f9bfaebd1f6b19e973fedd14e0090ce214`；应用 SHA、工作流及发布核验在发布完成后归档。审计目录 `/opt/adobe-relay/releases/adobe-safety-complete-20261015/`。

保留前版本镜像 `adobe-relay:before-adobe-safety-complete-20261015`。仅检查：

```sh
python3 /opt/adobe-relay/releases/adobe-safety-complete-20261015/adobe-safety-complete-release.py check-rollback
```

将末尾改为 `rollback` 才执行代码回退。脚本依赖同目录 `image-response-release.py`，核对应用版本、镜像内容、配置、设置和其他容器，调用正式 release.sh 切换应用；不恢复旧数据库，不绕过后续变更检查。

## 本地验证

带 race 的客户端、服务、图片 handler、Google 信封相关回归通过；新增构造响应覆盖 200/201/202/400/401/403/451/500、outputs 占位/链接、参考图拒绝、无凭据恢复/重复调用、OpenAI/Gemini 错误信封、流开始前/后、多行 SSE、普通错误及审核故障保留。客户 JSON/Python/bash 示例通过语法检查，回滚保护测试通过。

Adobe 客户端全包在清除当前终端 HTTP(S)/ALL_PROXY 环境变量后通过。之前三项本机下载测试失败来自测试进程继承环境代理后启用公网 DNS 防护；此次未修改下载安全策略或测试夹具。
