# 客户示例 usage 来源分析（2026-10-15）

## 结论与证据边界

能够确认：示例采用 OpenAI Images 兼容响应结构，图片 URL 指向 Adobe Firefly 的 S3 签名存储；输出 3336 可以由 OpenAI 官方 GPT Image 2 输出 token 计算器独立复算。无法确认：输入 2120 是模型服务返回、某个平台本地分词、加上隐藏提示词后的计数，还是人工填入；也无法仅凭此 JSON 判断整组 usage 是真实计量还是估算。

因此可以兼容字段结构，不能照抄这组固定数字作为每次调用的真实消耗。调查完成时采用真实 usage 或 null。用户随后明确选择随机模拟回填：真实 usage 保留，缺失时按 total_tokens 1800～3000 生成一致的明细，并标记 `usage_source: simulated`。这既不是取得了 Adobe 真实用量，也不是采用了输出估算公式；当前规则见 [客户调用说明](ADOBE_IMAGE_CLIENT_GUIDE.md)。

## 逐字段核对

| 字段 | 本例 | 能确认的内容 |
|---|---:|---|
| input_tokens | 2120 | 与 text_tokens 相等；缺少原始提示词、分词规则和请求处理记录，无法复算来源 |
| input_tokens_details.text_tokens | 2120 | 声称输入全部为文本；不说明是否包含平台改写或额外提示词 |
| input_tokens_details.image_tokens | 0 | 声称没有图像输入 token，不能仅靠输出图片判断其真实性 |
| output_tokens | 3336 | 与 gpt-image-2、3840×2160、medium 的官方计算器结果一致 |
| output_tokens_details.image_tokens | 3336 | 可直接由输出总量复制，单凭该字段不是额外计量证据 |
| output_tokens_details.text_tokens | 0 | 声称没有文本输出 token |
| total_tokens | 5456 | 仅是 2120 + 3336，不能独立证明两个来源都真实 |

## 3336 的可复算来源

[OpenAI 图像生成文档的输出 token 计算器](https://developers.openai.com/api/docs/guides/image-generation#cost-and-latency) 明确将其标为估算，并要求指定模型、尺寸和质量。2026-10-15 获取的计算器脚本对 gpt-image-2 使用 low / medium / high 档位常数 16 / 48 / 96。medium 的本例计算为：

```text
长边网格 = 48
短边网格 = round_even(48 × 2160 / 3840) = 27
图像网格数 = 48 × 27 = 1296
像素数 = 3840 × 2160 = 8294400
输出 token 估算 = ceil(1296 × (2000000 + 8294400) / 4000000)
                = ceil(3335.3856)
                = 3336
```

脚本在恰好 .5 时采用取偶数舍入；本例为整数 27，不涉及舍入歧义。复算代码：

```python
import math
long_grid = 48
short_grid = round(long_grid * 2160 / 3840)
print(math.ceil(long_grid * short_grid * (2_000_000 + 3840 * 2160) / 4_000_000))
# 3336
```

同一公式的 1024×1024、medium 得到 1756，说明 3336 不是固定模型常量，也不是 PNG 字节数换算。输入 token 与流式 partial images 不包含在这一输出估算中。该数值吻合说明平台可以不读取 Adobe usage 也算出 3336；但真实模型输出 token 也可能遵循相同计量，因此不能反向证明示例必定是伪造或估算。

本地证据脚本 SHA-256：`39fb64f4ed81ae1be38f4c3a08552afddd7de88c111565824b4e7bdbeb72e2e7`。未将该公式用于服务端 usage 或计费。

## 图片 URL 和时间能说明什么

- URL 的主机为 `pre-signed-firefly-prod.s3-accelerate.amazonaws.com`，与 Adobe Firefly 产物链路一致。其存储来源不证明 token 的统计来源。
- `x-resource-length=13279849` 表示资源长度信息，与模型 token 不是同一单位。
- `created=1788430668` 为 2026-09-03 10:17:48 UTC；签名时间参数为 10:17:47 UTC，相差 1 秒，时间上自洽。本例 `X-Amz-Expires=86400` 声明的签名期限是 24 小时，不代表其他返回链接都采用这个期限，也不保证底层临时凭证不会更早失效。
- AWS 签名约束 URL 请求，不为外围 JSON 的 usage 字段背书；没有原始调用地址、HTTP 回执、提供方实现或同任务用量记录，无法追溯这组数字的生产者。

[OpenAI Images API 的 usage 字段说明](https://developers.openai.com/api/reference/typescript/resources/images/methods/generate) 可证明结构兼容性；结构相似不等于来自 OpenAI 官方请求，也不等于 Adobe 透传了该字段。

## 本项目 Adobe 链路的实际证据

2026-10-14 两次 gpt-image-2 成功生成和后续任务轮询未见标准 usage。2026-10-15 又以已有 Token 和原代理生成一次：提交、10 次轮询及成功后的 6 次结果查询均未出现 token 用量，交付图片 973020 字节。使用临时诊断程序，未修改业务流程；见 [原始适配说明](ADOBE_IMAGE_RESPONSE_USAGE.md)。

公开 Firefly 前端中核对了 46 个有效脚本，发现 credits 成本查询及消费状态相关接口，未发现 input_tokens / output_tokens / total_tokens 明细。credits 是 Adobe 的额度单位，不能直接换成 token。检查范围不是 Adobe 所有内部接口；响应头仅保存名称，没有保存值，因此不能宣称所有响应头都已排除真实计量字段。

若以后取得原始示例调用平台，最有价值的比对是相同模型/尺寸/质量下改变提示词：保留同任务原始上游响应、usage 和平台算法，对照输入计数及输出公式。当前缺少平台信息，继续重复生成图片也不能验证该平台如何得出 2120。本轮复用已有成功回执和官方公开资料，没有新增付费出图。
