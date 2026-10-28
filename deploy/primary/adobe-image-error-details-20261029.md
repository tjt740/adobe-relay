# Adobe 图片错误细分已上线（2026-10-29）

用户授权合并、推送及部署。功能 [PR #130](https://github.com/tjt740/adobe-relay/pull/130) 已合并至 main `df69cbdd96d5e9c3f0e5c0714a0328df8171f1ac`。主服务器实际应用／镜像 **`66d9734be3d67baa4fa47cb2df505f0bb7f31193`**，镜像 ID `sha256:d722c062a69b909c691a863a4540543a6b865fa92c461b43d1ce7fd330a6ff64`。网站中英文每日日志已归入实际首次接流量的北京时间 **2026 年 10 月 29 日**，不使用开发日期。

正式后端、549 项前端、精确镜像及产物门禁 [40758174046](https://github.com/tjt740/adobe-relay/actions/runs/40758174046) 全部成功。合并／辅助／归档使用 `[skip ci]` 避免重复部署；核验以实际应用 SHA 为准。功能之外的后续提交只归档说明及核验辅助脚本，应用源码、Dockerfile 和正式构建工作流保持与实际应用一致。

## 接口变化

保留原 OpenAI 错误信封及既有 `type`、`message`，补充以下字段：

- `code`：网关定义的稳定错误码。
- `param`：有明确依据时提供的参数名；未知时省略。
- `stage`：validation、prepare、input_fetch、upload、submit、poll、download；无法判定时为 unknown。
- `source`：request、gateway 或 upstream，表示错误证据的来源。
- `upstream_status`：可取得的原始上游 HTTP 状态，网络未收到响应时省略；可能为 200，因为 HTTP 成功不代表任务成功。
- `request_id`：入口中间件生成或规范化的网关请求 ID，与 X-Request-ID 响应头一致；没有中间件提供时省略。

Gemini 保留数值 `error.code` 和 Google `error.status`，通过 `error.details` 的 ErrorInfo 提供相同诊断字段，稳定错误码放在 `reason` 中。

## 分类示例

| 原因 | 客户端 HTTP | code 示例 | param / stage |
| --- | --- | --- | --- |
| 空请求体、JSON 或 multipart 解析失败 | 400 | empty_request_body / invalid_json / invalid_multipart | validation |
| 图片数量、输出格式、背景组合不支持 | 400 | invalid_n / unsupported_output_format / incompatible_image_background | 相关参数 / validation |
| 不支持的模型、比例或尺寸限制 | 400 | unsupported_image_model / unsupported_aspect_ratio / image_size_exceeds_model_limit | model、aspect_ratio 或 size / prepare |
| 上游明确要求宽高是 16 的倍数 | 400 | image_size_not_multiple_of_16 | size / submit |
| 未识别的上游参数拒绝 | 400 | upstream_request_rejected | 不猜测参数 / 实际阶段 |
| 上游依赖失败 424 | 502 | upstream_dependency_failed | 实际阶段 |
| 上传缺少 ID、提交缺少任务地址或结果结构异常 | 502 | upstream_invalid_response | upload / submit / poll |
| 已受理任务最终失败 | 502 | upstream_generation_failed | poll |
| 已生成图片下载被拒绝 | 502 | upstream_image_download_failed / upstream_resource_unavailable | download |
| 输入图片网络获取失败、超时 | 502 / 504 | input_image_fetch_failed / input_image_fetch_timeout | images / input_fetch |
| 本地初始化、编码或传输配置故障 | 500 | image_gateway_not_initialized / image_payload_encoding_failed / image_transport_configuration_invalid | prepare 或实际阶段 |

已识别的上游字段只通过受控规则提取；不原样回传供应商正文、Cookie、Token、内部地址或签名图片链接。上传阶段保留上游请求 ID 与耗时；后台诊断增加 code、param、source，保留原脱敏证据。

## 兼容与保护

错误呈现和状态映射独立于是否允许提交。原 `CanRetrySubmission`、明确未受理判断、单来源提交预算、提交重试预算以及已受理／结果未知停止换号的保护保持；新增 502/504 仍使用终态错误，不自动重新 POST。下载错误只处理同一产物 GET，不能生成另一张图。

终态响应同时发送 `x-should-retry: false`，并在错误详情中提供 `retryable=false`、`resubmit_allowed=false`。这些字段是调用方停止自动重放的明确约定，不能替代服务端保护，也不能约束忽略标记的外部客户端。

不改变模型参数、图片尺寸、账号凭据、人工暂停、代理归属、并发、资源、SMTP、告警、持久配置或数据库。历史错误不重写。当前客户端、反向代理或调用方若仅按 5xx 自动重放 POST，须同时遵守图片生成的既有防重复提交约定；HTTP 状态本身不构成安全重放证明。

## 验证范围

回归覆盖 SDK 实际请求路径、网关分类和 OpenAI/Gemini 错误输出，核对上游状态与具体阶段，包含恶意上游正文不泄漏、缺字段、任务失败、下载 4xx、输入图策略与网络错误、参数校验在网络之前完成、生成 POST 次数及已受理／未知结果不重提。中英文日志通过完整性、日期归档保护、类型与构建检查。

上游失败和重试路径使用隔离模拟验证；线上另做一条在账号选择之前拒绝的无效参数请求。两者均不作为生产出图、全池恢复或真实高峰证明。

本地验证结果：Adobe SDK 全包 race 回归、Google 状态映射及 Adobe／图片相关 service、handler race 回归通过；错误响应与 Ops 归因相关 handler 回归通过。后端 `go build -p 2 ./...` 通过。前端每日日志与语言键完整性共 9 项通过，修改文件 ESLint、Vue 类型检查及生产构建通过。构建仍有既有的大分包提示。

首次扩大回归时，本机 HTTP_PROXY／HTTPS_PROXY 使回环测试服务被下载目的地保护拒绝；仅在测试子进程中去除代理环境变量后重跑通过，未修改宿主机或应用代理配置。未调用生产生成、登录或测试邮件。

## 发布与核验

| 步骤 | 北京时间（2026-10-29） | 证据 |
| --- | --- | --- |
| 临时实例接新连接 | 04:10:03 | rolling-route-canary.json |
| 旧侧自然排空完成 | 04:14:25 | 262 秒；请求槽、输入连接、自动检测均为零 |
| 正式实例接回新连接 | 04:14:58 | rolling-route-canonical.json |
| 临时容器／NAT／滚动租约清理 | 04:19:22 | rolling-finished.json |
| 最终核验通过 | 04:28:14 | final-verification.json |

准备 [40760113068](https://github.com/tjt740/adobe-relay/actions/runs/40760113068)、启动 [40760747601](https://github.com/tjt740/adobe-relay/actions/runs/40760747601)、最终核验 [40763511085](https://github.com/tjt740/adobe-relay/actions/runs/40763511085) 成功。原 44 项发布保护、本期 27 项辅助守卫通过；精确变更清单拒绝额外后端、迁移、调度、启动、Docker、法律或正式构建流程改动。提交入口、单来源预算、失败换号与已受理／未知结果停止重放的保护源码对比旧实际应用均保持。

首次核验 [40762371318](https://github.com/tjt740/adobe-relay/actions/runs/40762371318) 在错误接口辅助脚本的 SQL 聚合排序处失败，尚未发送测试 HTTP 请求。保留原日志及 `error-contract-verify.initial.py`，修复核验查询并使用实际响应头请求 ID 关联；修复前后哈希见 `verification-helper-repair.json`，没有改运行应用、配置或重做切流。补跑成功。

线上使用既有密钥发送一条 `n=0` 的无效输入，返回 HTTP 400、`code=invalid_n`、`param=n`、`stage=validation`、`source=request`、`message=n must be greater than 0`，响应体与 `X-Request-ID` 一致且没有虚构上游状态。只读核对该请求用量记录和已选账号记录均为零，未提交 Adobe 生成任务。两站 HTTPS、实际新资源、实时流与清理后正式进程告警心跳通过；两分钟 13 次被动采样均健康，0 OOM／重启。未进行本次生产认证浏览器／手机布局验收，资源和 API 检查不替代该项；未主动生成、登录、推送 Cookie 或发送测试邮件。

## 配置、数据与回退边界

- 实际切换备份与业务核验均为 **514 个未删除账号**，ID、状态、调度、人工暂停和手动代理模式保持；0 代理绑定变化，0 Cookie／Token／其他凭据变化。保留当前业务，不恢复旧数据或重写历史错误。
- Sub2 配置、环境、SMTP／告警、8 个 Clash 来源与节点／绑定配置、Okad 设置及其他容器保留；327 个已安装迁移及校验值完全一致，无新增迁移，已应用 255 保留，退役 408 门槛为零。
- 1290 在发布前及实际切换备份中已经删除，代理行和排除行均不存在，且零账号绑定／检测租约；本次保持真实基线，不重建旧节点或套用历史 active 断言。订阅清单和独立路由守卫均验证此状态。
- Sub2 保持 **54 GiB / GOMEMLIMIT=43008MiB / GOMAXPROCS=16 / CPU shares=4096 / 图片预算 37.8 GiB**。Okad 保持 **3 GiB**、原容器及发布前实际镜像 `adobe-team-cookie:28985c88b80f06ae276858596f2c54093f915161`，不覆盖为历史镜像；多 Token 和现有人工设置保留。
- 审计目录 `/opt/adobe-relay/releases/adobe-image-error-details-20261029/`；切换备份 `/opt/adobe-relay/backups/20261028T201426Z-66d9734be3d67baa4fa47cb2df505f0bb7f31193`。只读回退条件检查通过，未执行回退。
- 回退标签 `adobe-relay:before-adobe-image-error-details-20261029` 对应旧实际 `df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c`。真正回退前须重新审查后续版本、配置和在途任务并受控排空；只回代码，保留当前账号、凭据、代理和配置，不恢复旧数据库。旧应用会恢复较笼统的错误呈现。
