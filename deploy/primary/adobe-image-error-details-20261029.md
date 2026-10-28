# Adobe 图片错误细分（待发布）

本次基于 main `af50a04c35f8158473b1270aab29fae3997fc461`，尚未发布。网站中英文日志加入 `pendingDailyUpdates`，不使用开发日期冒充实际上线日期。发布时须按实际北京时间归入当天。

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

回归覆盖 SDK 实际请求路径、网关分类和 OpenAI/Gemini 错误输出，核对上游状态与具体阶段，包含恶意上游正文不泄漏、缺字段、任务失败、下载 4xx、输入图策略与网络错误、参数校验在网络之前完成、生成 POST 次数及已受理／未知结果不重提。中英文待发布日志须通过完整性、日期归档保护、类型与构建检查。

全部使用隔离模拟上游，不作为生产出图、全池恢复或真实高峰证明。正式部署、精确产物门禁及上线观察尚未执行。

本地验证结果：Adobe SDK 全包 race 回归、Google 状态映射及 Adobe／图片相关 service、handler race 回归通过；错误响应与 Ops 归因相关 handler 回归通过。后端 `go build -p 2 ./...` 通过。前端每日日志与语言键完整性共 9 项通过，修改文件 ESLint、Vue 类型检查及生产构建通过。构建仍有既有的大分包提示。

首次扩大回归时，本机 HTTP_PROXY／HTTPS_PROXY 使回环测试服务被下载目的地保护拒绝；仅在测试子进程中去除代理环境变量后重跑通过，未修改宿主机或应用代理配置。未调用生产生成、登录或测试邮件。
