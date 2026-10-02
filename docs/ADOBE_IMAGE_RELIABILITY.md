# Adobe 图片请求的故障定位与恢复

## 服务器行为

- `/v1/images/generations` 和 `/v1/images/edits` 的原生 Adobe 链路分别记录 `submit`、`poll`、`download` 阶段失败。
- `adobe_images.upstream_failed` 日志保留实际上游 HTTP 状态、错误类型、阶段耗时、尝试次数、代理 ID、上游 request ID 和 job ID（上游提供时）。网络错误的 `upstream_http_status=0`，不再把合成的客户端 502 当作真实上游 HTTP 状态。
- 后台运维错误详情同步保存该次上游错误。错误消息会脱敏和截断，不保存凭据及带签名的图片下载地址。
- 图片下载遇到网络临时错误或 HTTP 408/429/5xx 时，对同一地址最多尝试三次，间隔 1 秒、2 秒；权限拒绝、大小超限等终态错误不重试。已有下载大小限制继续生效。
- 已接受任务的轮询或下载临时错误耗尽重试后，停止跨账号重新生成，避免为恢复交付创建另一张收费图片。提交阶段保留原有切换账号策略。
- 生成成功后写响应失败会记录 `adobe_images.delivery_failed`，包含预计响应字节数、已写字节数和生成耗时。客户端已经断开会单独记录；用量仍遵循原有生成后记账逻辑。

## 调用方注意事项

同步调用需要保持连接直至完整结果返回。客户端、调用方中转、CDN 和反向代理的等待限制都需要检查。服务器的 `proxy_read_timeout=900s` 无法延长调用方自己设置的 60 秒总超时。

诊断时可使用 360 秒总超时并保存完整响应。以下操作会真实生成一张图片：

```sh
curl -sS --max-time 360 \
  -D headers.txt -o response.json \
  -w '\nHTTP=%{http_code} time=%{time_total}s bytes=%{size_download}\n' \
  "$CC_BASE/images/generations" \
  -H "Authorization: Bearer $CC_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-image-2.5-flare","prompt":"a red apple on a desk","n":1,"size":"3840x2160"}'
```

响应头 `X-Request-ID` 用于在本站日志里关联请求；它与 Adobe 的上游 request ID 不同。不要使用 `head` 截断图片响应。

2026-09-29 实测 `2048x2048` 和 `3840x2160` 可完整交付。修复日志后，Adobe 明确说明 flare 最长边不超过 3840 像素、总像素不超过 8,294,400；`4096x4096` 同时超过两项限制。原生 flare 现在提前校验这些限制并返回清楚的 400 提示，不提交无效任务，不自动缩图或改变比例，也不把该限制套用到其他模型。

当前版本现有异步图片接口只支持 OpenAI/Grok 分组并依赖对象存储，Adobe 原生账号不能直接照搬该接口来解决客户端断连。

## 验证

单元测试覆盖：下载临时错误恢复、总重试次数、不可重试错误、取消、下载大小上限、提交原始状态与 request ID 保留、凭据脱敏、运维上下文状态清理、已受理任务不重复生成，以及图片生成与响应写出失败的分别记录。
