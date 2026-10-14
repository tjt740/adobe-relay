# Adobe 图片 URL 与用量响应适配

## 上游核验（2026-10-14）

使用主服务器当前 Adobe 客户端、一个近期成功且启用的账号、该账号已有 Token 和原代理，独立验证 `gpt-image-2` / `1024x1024` / `medium`。两次各生成一张图片，均成功下载；未触发登录、Cookie 刷新、账号切换、管理接口检测或业务数据库写入。验证使用临时进程，没有发布或重启应用。

| 验证 | 北京时间 | 观察 |
|---|---|---|
| 第一次 | 19:34:45–19:35:12 | 首个可下载结果进度为 80，图片 732098 字节；响应没有 token 用量 |
| 第二次 | 19:37:30–19:39:03 | 图片 737902 字节；下载后又查询同一任务 12 次、间隔 5 秒，后续结果增加 `creativeCloudFileId`、不再带进度和轮询链接，仍没有 token 用量 |

后续响应包含 `contentType`、`generationSessionId`、`modelId`、`modelVersion`、`outputs[].image.{creativeCloudFileId,id,presignedUrl}`、`outputs[].seed` 和 `size`。未观察到 `usage`、`input_tokens`、`output_tokens` 或同义 token 统计。字符串全部在诊断进程内脱敏；[响应结构夹具](../backend/internal/service/testdata/adobe_image_response_20261014.json) 保留真实字段和数值，不含账号凭据、用户提示词或可用签名链接。

结论限定于本次模型、参数、账号和 Firefly 直连路径：当前不能取得可核实的真实 token 用量，不能证明其他提供方示例里的数字来自同一路径，也不把其他模型或未来上游行为一概判定为不支持。响应头诊断仅保留字段名，没有保存任意响应头值。

## 客户端调用

```json
{
  "model": "gpt-image-2",
  "prompt": "你的提示词",
  "size": "3840x2160",
  "quality": "medium",
  "output_format": "png",
  "response_format": "url"
}
```

调用原 `/v1/images/generations` 或 `/v1/images/edits`。下面是结构示意，时间、尺寸和链接均为示例，不是生产接口回执：

```json
{
  "background": "auto",
  "created": 1791977685,
  "data": [{"url": "https://example.invalid/temporary-image-url"}],
  "model": "gpt-image-2",
  "moderation": "auto",
  "output_format": "png",
  "quality": "medium",
  "size": "3840x2160",
  "usage": null
}
```

`null` 明确表示无法取得真实用量；客户端不应把它按零消耗处理。用户提供的标准用量结构仅作为解析兼容测试输入，不作为 Adobe 实测数据。解析只接受显式的标准 `usage`（任务顶层或唯一输出项），校验非负整数、精度上限、总量与明细的一致性；不递归猜测其他字段。若以后实际响应采用其他命名，需要另行核对和适配。

## 兼容范围

- 不传 `response_format` 时交付路径保持原样；显式 Base64 跳过存储。
- 显式 `url` 优先本站存储，其次已下载验证过的上游 HTTPS 链接；只有字节未被转码或重压缩改变时才使用原链接。不满足时仍交付完整 Base64 图片。
- 多图按请求顺序返回。全部图片有合法总量才汇总 `usage`，任一张用量缺失或异常时为 `null`。明细仅保留全部图片均已知的部分。
- `size` 和 `output_format` 从交付字节读取，不虚报请求尺寸或失败的转码结果。`quality` 和 `background` 描述提交档位；`moderation: auto` 保留原审核路径，不新增审核参数转发。
- 按张/分辨率计费、并发、失败换号、取消保护、Cookie 恢复、Gemini 信封、API Key 中转、视频和现有对象存储配置不改。没有数据库迁移。
- 正常生成不增加 Adobe 请求。额外轮询仅用于此次独立核验，不加入业务流程。临时 URL 不写入新增日志。

## 验证与发布边界

回归覆盖 URL/Base64/存储成功与失败、转码后链接一致性、实际尺寸、未知与非法用量、多图汇总及部分明细、数值上限、原计费字段和真实脱敏响应。已有 Adobe 客户端、服务、网关及路由相关测试继续运行。

验证结果（以下数量包含 Go 子用例）：

- 服务 331、网关 20、路由 17 项 `-race` 检查通过，共 368 项，无数据竞争报告。
- Adobe 客户端全包 `-race` 检查 445 项通过，3 项既有下载测试失败：`TestPlainTransportRoundTrip`、`TestPlainTransportTimeout`、`TestPlainTransportRejectsOversizedBody`。这三个测试用本机 HTTP 回环服务器，先被既有下载目标校验拒绝；在未修改的 main 上运行同样三个测试，得到相同失败。本次没有放宽生产下载校验来绕过它们。
- 基础 Adobe 服务、客户端生成、网关及路由回归通过；`git diff --check` 通过。

可复现命令（仓库 `backend/` 下执行）：

```sh
go test -race -tags unit ./internal/service ./internal/handler ./internal/server/routes -run 'Test.*Adobe|TestTranscodeAdobe|TestGenerateImage|TestGateway.*Adobe'
go test -race -tags unit ./internal/pkg/adobe
```

本次代码在独立分支 `codex/adobe-image-url-usage` 完成，用户已授权合并 main、推送并部署主服务器。发布状态、实际应用版本和回滚方式见 [发布记录](../deploy/primary/adobe-image-url-usage-20261014.md)。上线仅使用主服务器正式发布流程，先核对并行任务、现有应用版本与配置；不覆盖其他任务的 Okad 更新。
