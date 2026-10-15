# 图片接口客户调用说明

适用于 `https://creative.cheap` 的 Adobe 图片分组。使用本站分配的 API Key；示例中的 Key、图片链接、时间和用量数字均为占位或结构示例，不是生产回执。默认使用同步、非流式调用。

## 1. 文生图

在终端设置自己的 Key，不要放进网页前端代码或公开仓库：

```bash
export CREATIVE_API_KEY='替换为你的本站 API Key'
```

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/generations' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2",
    "prompt": "白色背景上的蓝色陶瓷杯，柔和自然光，产品摄影",
    "n": 1,
    "size": "3840x2160",
    "quality": "medium",
    "output_format": "png",
    "response_format": "url",
    "stream": false
  }' \
  -o response.json
```

`response_format: "url"` 请求返回图片链接；`n` 控制图片张数。Key 所属分组需要允许该模型和图片生成。请求可能耗时较长，客户端断开不等于上游取消，不要对超时请求无条件自动重发。

## 2. 成功返回与图片 URL

HTTP 200 示例（上游缺少真实用量，以下 usage 为随机模拟数字）：

```json
{
  "background": "auto",
  "created": 1788430668,
  "data": [
    {"url": "https://example.invalid/temporary-image.png?signature=EXAMPLE"}
  ],
  "model": "gpt-image-2",
  "moderation": "auto",
  "output_format": "png",
  "quality": "medium",
  "size": "3840x2160",
  "usage": {
    "input_tokens": 700,
    "input_tokens_details": {
      "image_tokens": 0,
      "text_tokens": 700
    },
    "output_tokens": 1700,
    "total_tokens": 2400,
    "output_tokens_details": {
      "image_tokens": 1700,
      "text_tokens": 0
    }
  }
}
```

第一张图片的 URL 在 `data[0].url`；多图遍历 `data`。`created` 是 Unix 秒，`size` / `output_format` 以实际交付图片为准。`quality` 表示提交档位，`moderation: "auto"` 表示默认审核方式。

```bash
jq -r '.data[] | select(.url != null) | .url' response.json
```

链接可能是本站存储地址，也可能是上游签名地址。签名 URL 有有效期，应尽快下载保存；不要删除、重新拼接查询参数，不要假定所有链接都固定有效 24 小时。下载图片不需要携带本站 API Key。

若存储不可用，或图片经转码后不能安全复用原链接，接口保留原有兜底：该项返回 `b64_json`。因此即使请求 `url`，客户端也应兼容以下结构：

```json
{
  "created": 1788430668,
  "data": [{"b64_json": "此处是完整的图片 Base64 字符串"}],
  "model": "gpt-image-2",
  "output_format": "png",
  "size": "3840x2160",
  "usage": {
    "input_tokens": 700,
    "input_tokens_details": {
      "image_tokens": 0,
      "text_tokens": 700
    },
    "output_tokens": 1700,
    "total_tokens": 2400,
    "output_tokens_details": {
      "image_tokens": 1700,
      "text_tokens": 0
    }
  }
}
```

## 3. Python 完整示例

先安装 `requests`（`python -m pip install requests`），沿用上面的 `CREATIVE_API_KEY` 环境变量。

```python
import base64
import os
from pathlib import Path

import requests

response = requests.post(
    "https://creative.cheap/v1/images/generations",
    headers={"Authorization": f"Bearer {os.environ['CREATIVE_API_KEY']}"},
    json={
        "model": "gpt-image-2",
        "prompt": "白色背景上的蓝色陶瓷杯，柔和自然光，产品摄影",
        "n": 1,
        "size": "3840x2160",
        "quality": "medium",
        "output_format": "png",
        "response_format": "url",
        "stream": False,
    },
    timeout=(15, 600),
)
try:
    result = response.json()
except ValueError:
    raise RuntimeError(f"接口返回非 JSON 内容，HTTP {response.status_code}")

error = result.get("error")
if response.status_code == 451 or (
    isinstance(error, dict) and error.get("status_code") == 451
):
    raise RuntimeError("提示词或参考图片被内容审核拒绝，请修改后再提交")
if not response.ok or error:
    message = error.get("message", "请求失败") if isinstance(error, dict) else "请求失败"
    raise RuntimeError(f"HTTP {response.status_code}: {message}")

images = result.get("data", [])
if not images:
    raise RuntimeError("响应中没有图片")

extension = result.get("output_format", "png")
if extension not in {"png", "jpeg", "webp"}:
    extension = "bin"
for index, item in enumerate(images, 1):
    if item.get("url"):
        print(f"第 {index} 张图片 URL: {item['url']}")
        # 不把 Authorization 发给图片存储服务器。
        image_response = requests.get(item["url"], timeout=(15, 120))
        image_response.raise_for_status()
        image_bytes = image_response.content
    elif item.get("b64_json"):
        image_bytes = base64.b64decode(item["b64_json"], validate=True)
    else:
        raise RuntimeError(f"第 {index} 张图片缺少 url 和 b64_json")
    Path(f"image-{index}.{extension}").write_bytes(image_bytes)

usage = result.get("usage")
if usage is None:
    print("真实 token 用量未知；不能按零消耗处理")
else:
    print("注意：上游缺失真实用量时，这里为随机模拟数字，不能仅凭数值确认真实消耗")
    print("输入 tokens:", usage["input_tokens"])
    print("输出 tokens:", usage["output_tokens"])
    print("合计 tokens:", usage["total_tokens"])
    print("输入明细:", usage.get("input_tokens_details"))
    print("输出明细:", usage.get("output_tokens_details"))
```

## 4. 图生图

本地参考图片可通过 multipart 上传；不要自行设置 `Content-Type`，由 curl 生成边界：

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/edits' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -F 'model=gpt-image-2' \
  -F 'prompt=保留杯子的形状，将背景改为浅灰色摄影棚' \
  -F 'image=@reference.png' \
  -F 'size=3840x2160' \
  -F 'quality=medium' \
  -F 'output_format=png' \
  -F 'response_format=url' \
  -o response.json
```

也支持 JSON 输入参考图片：将调用地址改为 `/v1/images/edits`，在第 1 节 JSON 中加入 `"images": [{"image_url": "https://你的公开域名/reference.png"}]`。使用实际可公开访问的 HTTPS 图片地址；下载限制或不受支持的参数会返回参数错误。返回结果的 URL / Base64 / usage 读取方式相同。

## 5. usage 的含义

接口保持原有字段结构，不增加来源字段。`usage` 有两种来源：

- 上游确实提供有效 token 用量时，保留真实值，包括明确的 0 和已有部分明细。
- 上游缺少完整有效用量时，按约定随机生成模拟数字，用于展示/联调。这不是实测，也不是估算。

**单凭返回数值无法区分这两种来源，客户端不能直接把 usage 作为真实 token 账单。** 本站计费仍使用原图片张数/分辨率规则。

模拟规则：每份成功响应的 `total_tokens` 为 **1800～3000 的整数（含边界）**，输入与输出随机拆分该总量，不按图片张数倍增。文生图的输入 `image_tokens=0`；有参考图片时，输入明细随机拆分为图片和文本，两者相加等于输入总量。输出全为图片，`text_tokens=0`。所有明细相加一致。每次响应独立随机，可能重复；与提示词长度、图片尺寸及真实消耗没有对应关系。

图片 URL、模型、时间、质量、实际尺寸与格式不随机。已有真实 usage（包括明确 0 和部分明细）保持原值，不用随机数混补真实记录。本站原有按图片张数/分辨率的计费保持，模拟数字不写入真实 token 用量和计费字段。本说明针对 Adobe 原生图片响应；中转及其他协议沿用原上游语义。

截至 2026-10-15 的线上 4K 实测，Adobe 成功响应仍未提供真实 token 用量，因此当前 usage 通常是按上述规则随机补齐的模拟数字。旧版本或其他接口仍可能返回 `usage: null`，它表示未取得真实用量，不表示零消耗、免费或生成失败。

如果上游以后确实返回有效用量，兼容以下结构。下面沿用客户提供的数字，仅展示字段，不能作为本项目 Adobe 实测值：

```json
{
  "background": "auto",
  "created": 1788430668,
  "data": [
    {
      "url": "https://example.invalid/temporary-image.png?signature=EXAMPLE"
    }
  ],
  "model": "gpt-image-2",
  "moderation": "auto",
  "output_format": "png",
  "quality": "medium",
  "size": "3840x2160",
  "usage": {
    "input_tokens": 2120,
    "input_tokens_details": {
      "image_tokens": 0,
      "text_tokens": 2120
    },
    "output_tokens": 3336,
    "total_tokens": 5456,
    "output_tokens_details": {
      "image_tokens": 3336,
      "text_tokens": 0
    }
  }
}
```

`input_tokens` 为输入总量，`output_tokens` 为输出总量，`total_tokens` 为两者之和。明细区分图像与文本 token；未知明细不应自行补零。多图调用只有全部图片都有合法总量时才汇总 usage；任一张缺失或不合法，整次 usage 为 null。当前没有启用估算返参。数字来源核验见 [usage 来源分析](ADOBE_IMAGE_USAGE_PROVENANCE.md)。

## 6. 违规与其他错误

提示词、参考图或输出内容被明确审核拒绝，尚未开始写响应时统一 HTTP **451**。Adobe 直连示例：

```json
{
  "error": {
    "type": "adobe_content_rejected",
    "message": "Image content was rejected by the upstream safety filter"
  }
}
```

本站审核或第三方中转可能保留其原有 `error.type` / `error.code`，例如 `prompt_guard_blocked` 或 `content_policy_violation`；客户端优先判断 HTTP 451，不要依赖固定英文文案。应提示用户修改提示词或参考图，不应对同一违规请求循环重试。

普通参数错误仍为 400；无效 Key、权限不足、限流和服务故障保持原有分类，不能把所有 4xx 或所有上游 451 都推断为提示词违规。

本说明默认 `stream: false`，Adobe 直连当前不提供原生流式出图。兼容中转流若已发出图片/心跳并提交 HTTP 200，HTTP 协议无法再改变状态；后续违规在错误事件的 `error.status_code` 中给出 **451**。开启 JSON 心跳的兼容接口也需检查错误对象。完整成功应同时满足无 `error` 且存在图片，不能只看 HTTP 200。

Gemini 兼容入口仍采用 Google 错误信封：违规时 HTTP 451，`error.code: 451`、`error.status: "PERMISSION_DENIED"`。异步任务只在支持该功能的分组启用；提交前拒绝为 451，已接受任务的查询 HTTP 200 表示查询成功，任务失败需读取 `status: "failed"` 及 `http_status: 451`，不能当成图片成功。本项目 Adobe 直连请使用上面的同步接口。
