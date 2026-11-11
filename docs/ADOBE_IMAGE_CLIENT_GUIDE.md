# Adobe GPT Image 2 / 2.5 / 2.6 接口参数与客户调用说明

适用于 `https://creative.cheap` 的 Adobe 图片分组。使用本站分配的 API Key；示例中的 Key、图片链接、时间和用量数字均为占位或结构示例，不是生产回执。默认使用同步、非流式调用。


文档范围：本站 **Adobe 原生图片通道**，版本日期 2026-11-07。相同分组如配置了其他兼容中转，其蒙版、流式及扩展参数以该通道实际能力为准；不能仅凭 OpenAI 接口形状推断 Adobe 支持所有功能。

## 0. 参数参考与模型差异

### 0.1 地址、认证与请求方式

| 项目 | 内容 |
| --- | --- |
| 服务地址 | `https://creative.cheap` |
| 文生图 | `POST /v1/images/generations`，JSON |
| 静态图片编辑 / 参考图生成 | `POST /v1/images/edits`，JSON 或 multipart/form-data |
| 模型目录 | `GET /v1/models`，返回当前 Key 可见的目录；可见不等于每次上游请求都成功 |
| 认证头 | `Authorization: Bearer <本站 API Key>`；不传 Adobe Cookie 或 Adobe Token |
| JSON 内容类型 | `Content-Type: application/json` |
| multipart 内容类型 | 交给 SDK / curl 自动生成含 boundary 的请求头 |
| 返回方式 | 同步 JSON。Adobe 原生不支持流式；使用 `stream: false` 或省略 |
| 客户端超时 | 示例连接超时 15 秒、总等待 600 秒；这是客户端等待设置，不是完成时长保证 |

Key 必须属于有相应模型权限、可用账号及余额/额度的分组。把模型名替换成 2.6 不会自动为 Key 开通权限。请在服务端保存 Key，不要写入浏览器前端、公开代码或图片下载链接。

### 0.2 可调用模型与能力对照

| 对外 `model` | 静态文生图 | 静态单图 / 多图参考编辑 | `complexity` | GIF 文生图 | GIF 参考编辑 |
| --- | --- | --- | --- | --- | --- |
| `gpt-image-2` | 支持 | 支持 | 不支持 | 不支持 | 不支持 |
| `gpt-image-2.5-flare` | 支持 | 支持 | 不支持 | 不支持 | 不支持 |
| `gpt-image-2.5-sunburst` | 支持 | 支持 | 不支持 | 不支持 | 不支持 |
| `gpt-image-2.6-flare` | 支持 | 支持 | `low / medium / high` | 支持 | 本地明确拒绝 |
| `gpt-image-2.6-sunburst` | 支持 | 支持 | `low / medium / high` | 支持 | 本地明确拒绝 |

两款 2.6 与 Adobe 官网同名：**GPT Image 2.6 Flare**、**GPT Image 2.6 Sunburst**。新的接入使用表内 `gpt-image-2.6-flare` / `gpt-image-2.6-sunburst`。旧 `gpt-image-2.6-star-swirl` / `gpt-image-2.6-aurora` 分别作为相同变体的兼容名称，仍受分组与账号白名单限制；已有旧名称白名单可识别对应新名称，不跨变体放行。2.5 的 `gpt-image-2.5-prism` 是 Sunburst 的兼容名称。不要使用没有变体后缀的 `gpt-image-2.5` 或 `gpt-image-2.6`。

两款 2.6 支持原生 WebP。SVG 和流式出图仍不支持。静态输出支持 PNG/JPEG，2.6 另支持 WebP 和纯文字 GIF 文生图；2/2.5 不接受 WebP 输出，添加 `svg.model` 或更换模型别名不会开启 SVG。

### 0.2.1 两款 2.6 的线上实测支持范围

2026-11-07 通过本站正式图片接口分别验证了 Flare（`gpt-image-2.6-flare`）与 Sunburst（`gpt-image-2.6-sunburst`），当时的结果两款一致；WebP 的新增支持与发布前实测见下节。支持范围以本通道为准，不能直接照搬其他平台的参数列表。

| 参数 / 能力 | 当前支持情况 | 实测结果与限制 |
| --- | --- | --- |
| `size=3840x2160 + quality=high` | 两款均支持 4K High 静态 PNG | 2026-11-07 正式接口两次成功，逐文件核验真实 3840×2160 像素与完整性 |
| `quality=xhigh / max` | 两款均支持 Extra High / Max 4K 静态 PNG | 2026-11-07 上线后正式接口 4/4 成功；实际提交档位分别为 6 / 7，响应和完整 PNG 均核验，零降档 |
| `background=auto / opaque` | 支持 | auto 请求正常生成；opaque 的 PNG 实际为完全不透明图片 |
| `background=transparent` | 支持 PNG 透明背景 | 两款返回的 PNG 均有真实透明像素；与 JPEG 组合返回 400 / `incompatible_image_background` |
| `complexity` | 仅支持 `low / medium / high` | 三档均有成功输出；`none / xhigh / max` 均返回 400 / `invalid_complexity`。不传时沿用上游默认，不等同于关闭规划 |
| `output_format` | 支持 `png / jpeg / webp / gif` | WebP 由上游原生生成，按实际字节核验格式与尺寸；SVG 仍返回 400 / `unsupported_output_format` |
| `frame_count` | GIF 支持 2–16，默认 9 | 两款均验证 2、9、16 帧；1 和 17 返回 400。16 帧使用 `size=auto` |
| `frame_duration_ms` | GIF 支持至少 20ms，按 10ms 取整 | 实际逐帧验证 20ms、默认 200ms，以及传 25ms 得到 30ms；19ms 返回 400 |
| `output_compression` | JPEG 本地编码；WebP 上游编码 | JPEG 25、90 已核验；WebP 接受整数 0–100，省略沿用上游默认，不在本站重复压缩；PNG 不使用该值，GIF 传此字段返回 400 |
| `stream=true` | 不支持 | 两款均返回 400 / `invalid_stream`；省略或使用 false |
| `partial_images` | 不支持渐进预览 | 静态非流式请求传 2 仍只返回一张完整图片，不产生中间图；GIF 传正数返回 400 / `gif_partial_images_unsupported` |
| `svg.model` | 不支持 SVG 模型选择 | 与 `output_format=svg` 一起传仍被拒绝，不是启用 SVG 的开关 |

这里的“支持”已经核对实际文件，不只检查 HTTP 200 或响应标签。`complexity` 控制规划，`quality` 控制生成质量，`output_compression` 控制 JPEG 或 WebP 编码，三者不能互相代替。透明背景可使用 PNG 或两款 2.6 的 WebP，不支持 JPEG。


### 0.2.2 WebP 调用与实测范围

WebP 由 Adobe 原生生成，本站保留原始文件并识别实际格式与尺寸。

2026-11-11 使用实时只读获取的线上账号和当前代理配置，在本地候选服务完成 **3 次真实生成，3 次成功**；均为 1024×1024、Medium，每项只提交一次上游任务。Go 与 Pillow 均完整解码通过。

| 模型 | 背景 / 压缩参数 | 交付与实际文件 |
| --- | --- | --- |
| Sunburst | auto / 省略 | 原生 URL，无损 WebP，759,664 字节 |
| Flare | auto / 省略 | 原生 URL，无损 WebP，678,592 字节 |
| Flare | transparent / 70 | Base64，有损 WebP，96,986 字节，存在真实透明像素 |

Sunburst 的 transparent / 25 组合因 SSH 读取超时、一次本地代理 TLS 预检失败及候选账号状态变化，未进入生成阶段，不计作成功或上游格式拒绝。用户提供的 Sunburst 3840×2160 WebP 已下载并完整解码，但本次没有新发 4K 请求。编辑、多图、高档质量及生产扣费不在这三次实测范围内。这些是发布前的本地实测结果，不代表其他组合已经验收。

**发布后正式接口验收（2026-11-11）：** 通过 `creative.cheap` 另完成两次 1024×1024 / Medium 生成，均为完整 WebP，各只提交一次上游任务；用量、密钥额度和实际余额扣费一致。临时测试密钥、分组与授权已清理，现有客户价格保持。

| 模型 | 背景 / 压缩参数 | 实际交付 |
| --- | --- | --- |
| Flare | auto / 省略 | URL，796,114 字节，完整解码通过 |
| Sunburst | transparent / 25 | Base64，63,654 字节，完整解码且含真实透明像素 |

上述 Sunburst 组合是在发布后独立完成的正式接口验证，发布前未进入生成的本地预检记录仍保留。本次没有新发 4K、编辑、多图或高档质量请求。

两款 2.6 及其同变体旧别名支持 `output_format=webp`，`response_format` 可选 `url` 或 `b64_json`。下例使用与用户提供的成功图片相同的模型、4K 尺寸和 Medium 质量；可以省略 `output_compression` 以沿用上游默认，或传整数 0–100。请求透明背景时使用 `background=transparent`。文件大小不保证按压缩参数同比例变化。

```bash
curl https://creative.cheap/v1/images/generations \
  -H "Authorization: Bearer $SUB2_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2.6-sunburst",
    "prompt": "A red ceramic mug on a pale wooden table",
    "size": "3840x2160",
    "quality": "medium",
    "output_format": "webp",
    "response_format": "url",
    "n": 1
  }'
```

读取 `data[0].url`，或将 Base64 解码后保存为 `.webp`。WebP 原文件会直接交付，响应的 `output_format` 和 `size` 根据实际文件识别。模型映射到旧模型时在提交前拒绝；已生成后若上游返回非 WebP 文件，则明确报错，不换号重提。WebP 沿用原模型、分辨率和质量计费规则。

### 0.3 公共参数

下面的“支持”指请求在 Adobe 原生通道中的实际处理方式。JSON 中数字应传数字，布尔值传布尔值；multipart 表单中的值均使用对应的文本形式。

| 参数 | 类型 | 默认 / 可选值 | 适用范围与说明 |
| --- | --- | --- | --- |
| `model` | string | 省略默认 `gpt-image-2`；建议始终显式填写 | 使用上表完整模型名，受 Key 分组权限限制 |
| `prompt` | string | 请提供非空描述 | 所有模型；编辑时描述要保留与修改的内容 |
| `n` | integer | 默认 `1`，允许 `1–10` | 返回图片/动画的数量，不是 GIF 帧数；建议先用 1。大于 10 会返回 400 |
| `size` | string | 省略 / `auto`，或 `宽x高` | 小写 `x`，例如 `1024x1024`。具体尺寸限制见下节；不要写 `1K`、`2K`、`4K` 或 `16:9` 代替像素 |
| `quality` | string | `auto / low / medium / high / xhigh / max` | 省略、空值或 `auto` 使用 medium；不同模型上限见质量表 |
| `output_format` | string | 省略保留上游格式；`png / jpeg / jpg`；2.6 可用 `webp / gif` | `jpg` 归一化为 JPEG；WebP/GIF 必须显式传入。WebP 返回格式不符时明确报错，不伪装成功或重新生成；PNG/JPEG 转码失败可能保留原字节，应读取实际响应格式 |
| `response_format` | string | `b64_json / url`，建议显式传 | `b64_json` 强制内联 Base64；`url` 优先链接，但存储或原图链接不可用时回退 Base64。省略时按服务器存储配置决定 |
| `background` | string | 省略 / `auto`，可用 `opaque / transparent` | 两款 2.6 的透明 PNG 和不透明 PNG 已实测；本次不扩大为旧模型的透明支持保证。`transparent` 不能与 `jpeg/jpg` 一起使用；透明图明确选 PNG |
| `output_compression` | integer | JPEG 默认 `100`，建议 `1–100`；WebP 为 `0–100` | JPEG 在本站编码，范围外截到 1–100（0 按 1）；WebP 透传上游，省略保留上游默认，范围外提交前拒绝。不是模型生成质量；PNG 不使用该值，GIF 不能传此参数 |
| `stream` | boolean | 默认 `false` | 不支持 true，返回 400 / `invalid_stream`；保持 false 或省略 |
| `partial_images` | integer | 请省略 | 不支持渐进预览；静态非流式中不会产生中间图，GIF 对正值返回 400 |
| `complexity` | string | 省略 / JSON `null` 保留上游默认；`low / medium / high` | 仅两款 2.6；与 quality 独立，适用于静态生成、静态编辑及 GIF 文生图 |
| `frame_count` | integer | 默认 `9`，允许 `2–16` | 仅两款 2.6 且 `output_format=gif`；固定 896×896 最多 9 帧 |
| `frame_duration_ms` | integer | 默认 `200`，至少 `20` | 仅 GIF；毫秒，按 10ms 取整，编码上限 655350ms，详见 GIF 参数表 |

**不要把兼容字段等同于已支持的能力：** `moderation` 不改变 Adobe 审核，响应按 `auto` 表示；`style`、`input_fidelity`、`user` 等未提供本通道对应的生成控制保证。原生不提供 partial-image 流，`partial_images` 不应作为静态请求控制项，GIF 对正值明确拒绝。`seed`、`fps`、`duration`、`loop` 不是本接口已开放的 GIF 参数；请使用准确的 `frame_count` 与 `frame_duration_ms`。

### 0.4 `quality` 实际档位

两款 2.6 的 **Extra High 与 Max 是独立档位**：`quality=xhigh` 对应 Extra High（上游档位 6），`quality=max` 对应 Max（档位 7），成功响应分别返回 `xhigh`、`max`，不再限制为 High。旧版把两档限制到 High 的行为已修正。

2026-11-07 上线后，通过本站正式 `/v1/images/generations` 接口，对两款 2.6 的 `xhigh / max` 各做一次生成，4/4 成功。四张均完整解码为 3840×2160 PNG；逐项关联 HTTP 请求、用量及生成日志，确认 Extra High 实际提交档位 6、Max 实际提交档位 7，响应分别为 `xhigh`、`max`，每项一次上游提交、零降档、零重放。静态文生图样本不能代表高档位 GIF、图片编辑、全部账号或高峰可靠性。

官网的质量菜单对应 `quality`；`complexity` 是独立的构图规划参数，不支持 `xhigh / max`。客户端应使用本站的 `xhigh` 名称，不要直接传官网内部的 `extraHigh` 或显示文字 `Extra High`。

| 客户传入 | Image 2 | 两款 Image 2.5 | 两款 Image 2.6 |
| --- | --- | --- | --- |
| 省略 / 空 / `auto` | medium | medium | medium |
| `low` | low | low | low |
| `medium` | medium | medium | medium |
| `high` | high | high | high |
| `xhigh` | 达到 high 上限，返回 high | Extra High（6），返回 xhigh | Extra High（6），返回 xhigh |
| `max` | 达到 high 上限，返回 high | Max（7），返回 max | Max（7），返回 max |

六种名字可被接收，不代表每个版本有六种不同的上游质量档位。`complexity` 不会覆盖 `quality`：例如 `quality=low, complexity=high` 表示低生成质量、高构图规划强度。两款 2.6 的未知质量值返回 400 / `invalid_quality`，不会因拼写错误悄悄回落 low；若 2.6 的 xhigh/max 被账号或渠道映射到其他模型，返回 400 / `quality_model_not_supported`，不降档生成。其他旧模型的未知质量值仍沿用原 low 兜底。

Extra High / Max 的 4K 调用示例（两款模型可互换）：

```bash
curl "$BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-image-2.6-flare","prompt":"A blue ceramic mug on a wooden table","quality":"xhigh","size":"3840x2160","output_format":"png","response_format":"b64_json","n":1}'
```

```bash
curl "$BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-image-2.6-sunburst","prompt":"A blue ceramic mug on a wooden table","quality":"max","size":"3840x2160","output_format":"png","response_format":"b64_json","n":1}'
```

### 0.5 静态 `size` 与 4K 边界

| 模型 | 尺寸处理与已知限制 |
| --- | --- |
| Image 2 | 明确的宽高沿用原链路传给上游，不套用 2.6 新限制；最终以 Adobe 实际接受与返回为准 |
| Image 2.5 Flare | 最长边不超过 3840，面积不超过 8,294,400 像素；超出会在本地返回 400 |
| Image 2.5 Sunburst | 明确宽高沿用原链路，不新增 Flare/2.6 的本地限制；最终以上游结果为准 |
| 两款 Image 2.6 | 宽高均为 480–3840 且是 16 的倍数；总像素 655,360–8,294,400；长宽比不超过 3:1；违规返回 400，不自动改成其他尺寸 |

常用尺寸：`1024x1024`（方图）、`1536x1024`（横图）、`1024x1536`（竖图）、`2048x2048`（2K 方图）、`3840x2160`（横向 4K）、`2160x3840`（竖向 4K）。这些尺寸符合 2.6 静态校验规则；通过参数校验仍需有对应上游能力和额度。

`4K` 不等于 `4096x4096`。2.6 的 `4096x4096`、`512x512`、宽高不是 16 倍数或过长宽比的静态请求都不符合上述限制。`auto` 由 Adobe 选尺寸，不保证固定输出为 1024、2048 或 4K；响应 `size` 描述实际图片尺寸。GIF 的 auto 和固定尺寸使用下一节的专门规则，不能直接套静态尺寸。

### 0.6 GIF 参数、默认值与组合限制

| 参数 / 组合 | 实际行为 |
| --- | --- |
| 模型 | 仅 `gpt-image-2.6-flare`、`gpt-image-2.6-sunburst` |
| 端点与输入 | `/v1/images/generations`，纯提示词，不传参考图 |
| `output_format` | 必须为 `gif` |
| `frame_count` | 整数 2–16；省略或 JSON null 为 9；小数和超范围值返回 400 |
| `frame_duration_ms` | 整数且至少 20；省略或 JSON null 为 200；25ms 会取整为 30ms。本次验证 20、25→30 和默认 200ms；此前有 2000ms 成功样本。实现将超过 655350ms 的值截到 GIF 编码上限，不代表已实测长时延生成 |
| `size` 省略 / `auto` | Adobe 按帧数决定尺寸；支持 2–16 帧。本次两款默认 9 帧和 16 帧样本实际均为 512×512，但不承诺 auto 永远返回这个尺寸 |
| `size=896x896` | 仅允许 2–9 帧；默认 9 帧可用。需 10–16 帧时改为 auto |
| 其他显式尺寸 | 本通道 GIF 返回 400，包括 `1024x1024`、`512x512`、`640x640`；auto 输出 512×512 不代表允许显式传此尺寸 |
| 总动画时长 | 近似为 `frame_count × 实际取整后的 frame_duration_ms`，不是独立参数；4 帧 × 200ms 为一轮 0.8 秒 |
| `n` | 动画个数；一份 GIF 计一张图片，帧数不作为图片张数 |
| 不支持的组合 | GIF 编辑端点、参考图片、mask、流式、正数 partial_images、自定义 output_compression；不应发送这些组合 |

第三方示例中的 `896x896 + 4 帧 + 200ms` 可以直接沿用，模型名换成本通道的实际 2.6 名称。对“896×896 最多 8 帧”的第三方建议，本站以实测通过的 **最多 9 帧** 为准；16 帧请用 auto。支持范围之外的组合会明确拒绝，不会偷偷减少帧数或缩小客户指定的固定尺寸。

### 0.7 编辑输入参数与大小

| 写法 | 参数与要求 |
| --- | --- |
| JSON 单图 | `"images": [{"image_url": "https://example.com/ref.png"}]` |
| JSON 多图 | 在 `images` 数组中放多个 `image_url` 对象；每个对象是一张参考图；已验证双图，上限仍受整包大小和 Adobe 限制，不代表无限数量 |
| JSON 内联图 | `image_url` 可用 `data:image/png;base64,...` 或 `data:image/jpeg;base64,...`；不要只传不带前缀的 Base64 |
| multipart 单图 | 文件字段 `image`，例如 `-F 'image=@ref.png;type=image/png'` |
| multipart 多图 | 重复使用 `image[]` 文件字段，或 `image[0]`、`image[1]` |
| 格式 | 参考图使用 PNG/JPEG；2.6 能识别 SDK 以 application/octet-stream 上传的 PNG/JPEG；为兼容 2/2.5，请始终显式发送正确 MIME |
| 远程地址 | 公网 HTTPS 域名、443 端口，不能是 IP 字面量、内网地址、需要登录的页面或重定向地址 |
| URL / Data URL 大小 | 每张解码后不超过 20 MiB；远程抓取等待上限 30 秒 |
| multipart 大小 | 通用解析器每个 part 上限 50 MiB；整包和上游仍有自身限制。建议每张不超过 20 MiB，保证两种传法一致可用 |
| `file_id` | 不支持，使用 `images[].image_url` 或直接上传文件 |
| 蒙版 `mask` | Adobe 原生没有保证生效的真正局部蒙版编辑；混合分组会优先尝试有此能力的兼容中转。不能把原生参考图编辑当成蒙版能力；GIF 不支持 mask |

JSON 编辑请用 `images` 数组，不要只写 `image` 字符串。GIF 不允许把参考图放进生成接口来绕过编辑限制；这同样返回 `gif_edit_unsupported`。

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

### GPT Image 2.6 构图规划强度

`complexity` 仅可传 `low`、`medium` 或 `high`，不支持 `none`、`xhigh`、`max`，与 `quality` 独立；文生图 JSON、图生图 JSON 和 multipart 表单均支持。省略或传 `null` 时保留上游默认；非法值在提交前返回 400。2/2.5 和其他 Adobe 模型的原调用方式不变，不支持此新增字段。

```json
{
  "model": "gpt-image-2.6-sunburst",
  "prompt": "一个热闹的夜市街景，多个摊位和行人，暖色灯光，电影感",
  "size": "2048x2048",
  "quality": "high",
  "complexity": "high"
}
```

### GPT Image 2.6 GIF

`gpt-image-2.6-flare` 和 `gpt-image-2.6-sunburst` 支持原生 GIF 文生图。客户端仍调用同一个 `/v1/images/generations`，只需增加 GIF 参数：

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/generations' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2.6-flare",
    "prompt": "一只红色陶瓷杯做短循环动画",
    "size": "896x896",
    "output_format": "gif",
    "frame_count": 4,
    "frame_duration_ms": 200,
    "response_format": "b64_json"
  }' \
  -o response.json
```

`frame_count` 接受 2–16，省略时为 9；`frame_duration_ms` 接受不小于 20 的整数毫秒，并按 10 毫秒取整；GIF 编码时长上限为 655350 毫秒，省略时为 200。省略 `size` 或写 `auto` 时，由 Adobe 按帧数选择尺寸，支持 2–16 帧；16 帧已实测返回 512×512。需要固定尺寸时使用 `896x896`，此尺寸最多 9 帧，10–16 帧必须使用 `auto`；不支持的显式尺寸和超出拼图像素预算的组合会在提交前返回 400，不会静默缩小尺寸或减少帧数。GIF 按一个动画计一张图片，分辨率档位根据实际返回的单帧尺寸确定。GIF 仅支持无参考图的文生图；`/v1/images/edits` 或携带参考图/上传图的 GIF 请求在调用 Adobe 前返回 HTTP 400、`code: "gif_edit_unsupported"`、`param: "image"`。静态图编辑请使用 PNG/JPEG。GIF 也不支持流式、partial images、mask 或自定义 `output_compression`。成功后 `data[0].b64_json` 是完整 GIF 字节；若请求 URL 交付，客户端仍应兼容存储 URL 或 Base64 回退。

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

第一张图片的 URL 在 `data[0].url`；多图遍历 `data`。`created` 是 Unix 秒，`size` / `output_format` 以实际交付图片为准。`quality` 表示网关采用的归一化质量标签，`moderation: "auto"` 表示默认审核方式。GPT Image 2.5 请求 `xhigh` 返回 `xhigh`，请求 `max` 返回 `max`，两者按官网分别提交 `detailLevel=6 / 7`；2 / 1.5 请求这两档仍返回 `high`。该标签不是上游对成图质量的评估。

Adobe 原生 GPT Image 1.5、2、2.5、2.6 请求 `quality: "auto"`、省略 `quality` 或传空值时，默认提交 `medium` 对应的 `detailLevel=3`，成功响应返回 `quality: "medium"`。显式 `low` 仍提交档位 1；显式 `medium`、`high`、`xhigh`、`max` 及各模型上限保持。大小写和首尾空格会规范化；2.6 未知质量值明确拒绝，其他旧模型沿用原低档兜底。文生图和图片编辑使用同一规则；非 GPT Image 模型及中转账号保留原处理。账号检测显式使用 `low`，不随业务默认值提高。站内计费按张数、分辨率和已配置的 2 / 2.5 / 2.6 质量单价计算；未配置质量单价沿用原收费。较高生成档位对耗时与 Adobe 额度的影响以实际结果为准。

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
if extension not in {"png", "jpeg", "gif"}:
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
  -F 'image=@reference.png;type=image/png' \
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

**单凭返回数值无法区分这两种来源，客户端不能直接把 usage 作为真实 token 账单。** 本站计费使用图片张数、分辨率及可选的 2 / 2.5 / 2.6 质量单价，见第 8 节。

模拟规则：每份成功响应的 `total_tokens` 为 **1800～3000 的整数（含边界）**，输入与输出随机拆分该总量，不按图片张数倍增。文生图的输入 `image_tokens=0`；有参考图片时，输入明细随机拆分为图片和文本，两者相加等于输入总量。输出全为图片，`text_tokens=0`。所有明细相加一致。每次响应独立随机，可能重复；与提示词长度、图片尺寸及真实消耗没有对应关系。

图片 URL、模型、时间、质量、实际尺寸与格式不随机。已有真实 usage（包括明确 0 和部分明细）保持原值，不用随机数混补真实记录。本站按图片张数、分辨率及可选的 2 / 2.5 / 2.6 质量单价计费，模拟数字不写入真实 token 用量和计费字段。本说明针对 Adobe 原生图片响应；中转及其他协议沿用原上游语义。

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

`input_tokens` 为输入总量，`output_tokens` 为输出总量，`total_tokens` 为两者之和。明细区分图像与文本 token；未知明细不应自行补零。多图调用只有全部图片都有合法总量时才汇总真实 usage；无法得到有效汇总时，按本节所述规则使用模拟回退。当前并没有基于图片实际消耗的 token 估算。数字来源核验见 [usage 来源分析](ADOBE_IMAGE_USAGE_PROVENANCE.md)。

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

### 6.1 客户可判断的错误字段

```json
{
  "error": {
    "type": "adobe_request",
    "message": "GIF reference editing is not supported on the Adobe channel; use text-only /v1/images/generations for GIF output or png/jpeg for image edits",
    "code": "gif_edit_unsupported",
    "retryable": false,
    "resubmit_allowed": false,
    "param": "image",
    "stage": "validation",
    "source": "request"
  }
}
```

这是结构示例。终态错误可能同时携带 `x-should-retry: false`、`retryable: false`、`resubmit_allowed: false`，表示不要原样重放本次请求。客户端主要按 HTTP 状态、`code`、`param` 判断，不要匹配整段英文 `message`。`stage` 表示校验、输入抓取、上传、提交、轮询或下载等阶段；`source` 区分请求、上游或网关。只有存在实际上游状态时才有 `upstream_status`；`request_id` 如返回，应连同调用时间保存用于排查，不是可直接重试或查询任务的凭证。

| HTTP / `code` | 含义 | 客户处理 |
| --- | --- | --- |
| 400 / `invalid_n` | n 超出当前支持范围 | 使用 1–10 的整数 |
| 400 / `unsupported_output_format` | 请求 SVG，或在非 2.6 模型（含映射目标）上请求 WebP | 使用 PNG/JPEG；WebP 请选择两款 2.6；2.6 动图使用 GIF |
| 400 / `invalid_output_compression` | WebP 压缩参数不在整数 0–100 范围 | 修正参数或省略以沿用上游默认 |
| 502 / `invalid_image_output` | 已受理的 WebP 任务返回无效或非 WebP 文件 | 不要重试或重新提交同一生成请求 |
| 400 / `image_size_exceeds_model_limit` | 显式静态尺寸不满足模型规则 | 按模型尺寸表修改，勿把 4K 当作 4096 方图 |
| 400 / `invalid_complexity` | complexity 使用 none、xhigh、max 等无效值 | 仅使用 low / medium / high；不传时保留上游默认 |
| 400 / `invalid_stream` | 请求 stream=true | 使用 false 或省略；本通道不支持流式出图 |
| 400 / `complexity_model_not_supported` | 给 2/2.5 等模型添加 complexity | 移除该字段，或改用有权限的 2.6 |
| 400 / `gif_model_not_supported` | 对不支持的模型请求 GIF | 只用两款 2.6 |
| 400 / `gif_edit_unsupported` | GIF 携带参考图或调用编辑端点 | 改为无参考图的 GIF 文生图，或把编辑输出改为 PNG/JPEG |
| 400 / `gif_parameters_require_output_format` | 未请求 GIF 却传帧参数 | 加 output_format=gif，或删除帧参数 |
| 400 / `invalid_frame_count` | 帧数不在 2–16 | 修正帧数 |
| 400 / `invalid_frame_duration_ms` | 每帧时长小于 20ms | 设置不小于 20 的整数 |
| 400 / `gif_size_unsupported` | GIF 使用了其他显式尺寸 | 使用 auto 或 896x896 |
| 400 / `gif_frame_size_budget_exceeded` | 896x896 搭配 10–16 帧 | 改用 auto，或将帧数降到 2–9 |
| 400 / `gif_mask_unsupported`、`gif_partial_images_unsupported`、`gif_output_compression_unsupported` | GIF 使用不支持的组合 | 删除相应参数；原生请求保持非流式 |
| 400 / `incompatible_image_background` | transparent 与 JPEG 冲突 | 改为 PNG，或改 opaque/auto |
| 401 / 403 | Key、分组、模型权限或上游账户权限问题 | 核对 Key 所属分组和具体错误，不要反复提交生成 |
| 413 | 请求体过大 | 减小参考图片或整包尺寸 |
| 429 / 503 | 当前限流、容量或可用账号不足等 | 读取具体错误及服务器提示，不等于模型参数错误 |
| 451 | 内容审核拒绝 | 修改内容，不循环提交同一请求 |
| 5xx、客户端读超时或连接中断 | 请求可能已被受理，或输出/下载遇到故障 | 保留时间、模型和请求 ID，先核实结果，不能把同一生成无条件重发 |

字段类型错误通常在通用解析阶段直接返回 400，未必有上表中的模型专用 code。缺少可用账号、权限或余额时也可能先被入口拒绝，因此不能要求每个非法参数请求都一定到达模型校验阶段。

### 6.2 超时和重试

一次请求可能包含多个已经产生消耗的图片任务。服务端保留原有“已受理或结果未知不重放”保护；客户端也不要配置生成 POST 的通用自动重试。HTTP 400 的已知本地参数拒绝可以在修正输入后发起新请求；提交、轮询或下载阶段的失败，应先核查是否已生成，不应直接循环重发。设置客户端 request ID 或重复发送相同 JSON **不等于服务支持幂等生成**。

## 7. 可复制的调用配方

以下使用第 1 节设置的 `CREATIVE_API_KEY`。每条生成/编辑请求都可能产生实际消耗；选择所需示例单独调用，不需要把所有示例批量执行。

### 7.1 Image 2.5：两张 JPEG 静态图

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/generations' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2.5-flare",
    "prompt": "白底蓝色陶瓷杯，产品摄影",
    "size": "1024x1024",
    "quality": "high",
    "n": 2,
    "output_format": "jpeg",
    "output_compression": 90,
    "response_format": "b64_json"
  }' -o response.json
```

使用 Sunburst 时，仅将 model 改成 `gpt-image-2.5-sunburst`。不要给此请求添加 complexity 或帧参数。

### 7.2 Image 2.6：2K 高质量与高构图规划

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/generations' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2.6-sunburst",
    "prompt": "一个热闹的夜市街景，多个摊位和行人，暖色灯光，电影感",
    "size": "2048x2048",
    "quality": "high",
    "complexity": "high",
    "output_format": "png",
    "response_format": "url",
    "n": 1
  }' -o response.json
```

### 7.3 Image 2.6：16 帧 GIF

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/generations' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2.6-sunburst",
    "prompt": "一只小鸟挥动翅膀，白色背景，连贯循环动画",
    "size": "auto",
    "quality": "medium",
    "complexity": "medium",
    "output_format": "gif",
    "frame_count": 16,
    "frame_duration_ms": 200,
    "response_format": "b64_json",
    "n": 1
  }' -o response.json
```

这是一份动画，不是 16 张图片。成功返回 `data[0].b64_json` 时直接解码并保存 `.gif`，不要将其转成 PNG 后再交给客户，否则会丢失动画。也可替换为 `gpt-image-2.6-flare`。

### 7.4 Image 2.6：JSON 多图参考与 complexity

把示例地址替换为自己的公网 HTTPS 图片直链：

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/edits' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2.6-sunburst",
    "prompt": "保留第一张图中杯子的造型，采用第二张图的配色与背景",
    "images": [
      {"image_url": "https://example.com/reference-1.png"},
      {"image_url": "https://example.com/reference-2.jpg"}
    ],
    "size": "1024x1024",
    "quality": "high",
    "complexity": "high",
    "output_format": "png",
    "response_format": "b64_json"
  }' -o response.json
```

Image 2/2.5 使用相同 `images` 数组结构，但必须移除 complexity。多张参考图与 n 是两回事：前者是输入参考数量，后者是输出数量。

### 7.5 Image 2.6：multipart 多图参考

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/edits' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -F 'model=gpt-image-2.6-flare' \
  -F 'prompt=融合两张参考图，保持主体外形，使用统一摄影棚光照' \
  -F 'image[]=@reference-1.png;type=image/png' \
  -F 'image[]=@reference-2.jpg;type=image/jpeg' \
  -F 'size=1024x1024' \
  -F 'quality=medium' \
  -F 'complexity=high' \
  -F 'output_format=png' \
  -F 'response_format=url' \
  -o response.json
```

不要同时给 curl 手工添加 `Content-Type: multipart/form-data`，否则容易缺少 boundary。JSON 的 `null` 与表单字符串 `null` 不同；multipart 参数想使用默认值时直接省略字段，不要写 `frame_count=null`。

### 7.6 Image 2.6：透明 PNG

以下参数组合已在线上两款 2.6 验证。示例选 Flare，也可仅将 model 改为 `gpt-image-2.6-sunburst`。透明效果应读取 PNG 的透明通道，不应只看响应中的 background 标签。

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/generations' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2.6-flare",
    "prompt": "一把橙色陶瓷茶壶，主体完整，透明背景，无文字",
    "size": "1024x1024",
    "quality": "low",
    "complexity": "low",
    "background": "transparent",
    "output_format": "png",
    "response_format": "b64_json",
    "stream": false,
    "n": 1
  }' -o transparent-response.json
```

将 `data[0].b64_json` 解码后保存为 `.png`。需要不透明背景时改为 `background: "opaque"`；交给上游决定时用 `auto`。两款 2.6 也可将 output_format 改成 webp 并保存为 `.webp`；不要改成 jpeg 或 svg。

### 7.7 Image 2.6：JPEG 压缩质量

`quality` 与 `complexity` 保持独立。下面的 `output_compression=25` 控制 JPEG 编码；改为 `90` 可使用另一档已验证的编码质量。数值不是“文件缩小到原来的百分之几”，不能承诺固定文件大小。

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/generations' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2.6-sunburst",
    "prompt": "一把橙色陶瓷茶壶，简洁摄影棚背景，无文字",
    "size": "1024x1024",
    "quality": "low",
    "complexity": "high",
    "background": "auto",
    "output_format": "jpeg",
    "output_compression": 25,
    "response_format": "b64_json",
    "stream": false,
    "n": 1
  }' -o jpeg-response.json
```

解码后保存为 `.jpg` 或 `.jpeg`。透明背景不能用于 JPEG；PNG 不使用 output_compression，GIF 不能传此字段。`complexity=high` 不会把本示例的 `quality=low` 自动升为 high。

### 7.8 Image 2.6：GIF 帧时长取整

两款均已验证 `size=auto + 16 帧 + 25ms`，实际文件为 16 帧、每帧 30ms，本次单帧为 512×512。一轮动画约 0.48 秒；auto 的尺寸由上游决定，不保证每次固定为 512×512。

```bash
curl --fail-with-body --connect-timeout 15 --max-time 600 \
  'https://creative.cheap/v1/images/generations' \
  -H "Authorization: Bearer ${CREATIVE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2.6-flare",
    "prompt": "一片绿色叶子轻轻旋转，白色背景，连贯循环动画，无文字",
    "size": "auto",
    "quality": "low",
    "complexity": "medium",
    "output_format": "gif",
    "frame_count": 16,
    "frame_duration_ms": 25,
    "response_format": "b64_json",
    "stream": false,
    "n": 1
  }' -o animation-response.json
```

默认动画可省略 frame_count 和 frame_duration_ms，本次两款实际均为 9 帧、每帧 200ms。需要验证最小值时可改为 `size=896x896, frame_count=2, frame_duration_ms=20`，此组合也已通过。GIF 示例不能加入参考图、mask、partial_images 或 output_compression；也不能改用编辑端点。

## 8. 响应字段、计费与客户端保存

| 响应字段 | 类型 | 使用说明 |
| --- | --- | --- |
| `created` | integer | 响应创建时间，Unix 秒 |
| `data` | array | 遍历所有输出，不能只取第一项而丢掉 n > 1 的图片 |
| `data[].url` | string，可选 | 图片链接，尽快下载；不能假定固定有效期；下载时不携带本站 API Key |
| `data[].b64_json` | string，可选 | 不带 data URL 前缀的完整图片 Base64；解码后按实际格式保存 |
| `model` | string | 本次请求模型名称，使用推荐实际名称可避免别名歧义 |
| `quality` | string | 归一化的实际质量档位标签，按 0.4 节解释 |
| `output_format` | string | 实际交付格式；GIF 成功时为 gif |
| `size` | string | 实际图片/单帧宽高；多图尺寸不一致时可能不提供统一 size |
| `background` | string | 请求的规范化背景控制标签，不是图像内容检测报告 |
| `moderation` | string | 原生按 auto 处理，不能用它证明关闭了审核 |
| `usage` | object 或旧响应中的 null | 可能来自真实上游，也可能是展示用随机模拟值，不能直接作为真实 token 账单，见第 5 节 |

`complexity`、`frame_count`、`frame_duration_ms` 不保证在顶层原样回显。需要显示动画属性时，读取 GIF 自身的帧和延时信息；不能因为响应没有同名字段就认定参数未生效。

图片计费与模拟 usage 分开。明确静态尺寸沿用原来的长边分档：不超过 1024 为 1K、不超过 2048 为 2K、更大为 4K；静态 size 省略/auto 保留既有默认 2K 档位，所以需要确定档位时请显式指定 size。2.6 GIF 按实际输出的单帧长边确定档位，每个动画计一张；例如一份 512×512 的 16 帧 GIF 仍是一张 1K 动画。具体单价、倍率和余额以当前 Key 所属分组的站内计费配置为准。

### Image 2 / 2.5 / 2.6 分辨率与质量定价

分组创建支持多选模型，以 ` + ` 连接模型名。新版计费开关默认开启，编辑时回填已保存的模式；开启后使用所选模型的标签页，按分辨率行和质量列分别填写，支持四位小数。切换标签页保留草稿，创建仅提交当前所选模型的价格；编辑保留已有其他模型价格。模型名称不替代模型白名单或账号绑定。

管理员可进入 **分组管理 → 创建/编辑 Adobe 分组 → 图片生成计费 → 启用新版质量计费**。新建及未设置模式的 Adobe 分组默认新版；编辑回填已保存模式，明确保存的关闭状态保持。计费说明收进标题旁的感叹号，点击查看。开启并保存后优先按模型、分辨率及质量匹配单价，空白沿用旧版，0 免费；关闭并保存后完全使用旧版计费，仍保存并保留已填写的质量价格。再次开启可恢复。Image 2 表格仅显示官网默认尺寸对应的 1K × Low / Medium / High；2.5/2.6 的 Flare、Sunburst 各为三种分辨率 × 五档质量。分组字段 `image_quality_pricing_enabled` 与 `image_quality_prices` 独立保存；更新省略模式字段时保持原值。

2026-11-09核对 Adobe 官网公开模型schema及当前前端转换规则：Image 2 上限5，2.5/2.6上限7，Extra High / Max分别为6 / 7。官网2.5/2.6列出1K/2K/4K；Image 2官网采用默认比例尺寸、没有独立分辨率菜单，其公开尺寸schema覆盖本站三个计费档位。此核对不等于真实生成或扣费验收。

- 只有精确匹配的模型、分辨率和实际质量格子有价格时才覆盖原价；空白沿用原有逐模型、分组、渠道或内置价格优先级，清空后恢复原收费。**0 表示免费，空白不等于 0**。不会自动预填或修改任何现有客户价格。
- 总标准费用 = 对应单价 × 图片张数；实际扣费再乘以适用的图片倍率。已开启“生图倍率独立”时用独立倍率，否则沿用当前分组有效倍率。
- `quality=xhigh` 对应 Extra High，`quality=max` 对应 Max；`auto`、省略或空质量按实际采用的 Medium 匹配。未知质量仍按0.4节规则处理；计费不会改变生成档位。
- 旧 `gpt-image-2.6-star-swirl` 与 Flare 共用价格，旧 `gpt-image-2.6-aurora` 与 Sunburst 共用价格。2.5 的 prism 与 Sunburst 共用价格；其他模型及缺少实际质量信息的中转响应继续沿用原收费。
- 成功用量记录新增可选 `image_quality`（`low` / `medium` / `high` / `xhigh` / `max`），与已有 `image_size`、`image_count`、标准费用、实际费用一起显示；历史记录不补猜质量，也不重新计费。图片响应继续使用原有 `quality` 字段，客户端不必更改请求格式。
- 此价格表针对 Adobe 原生 2 / 2.5 / 2.6 的静态生成、编辑及2.6已支持的 GIF；GIF 仍按每个动画一张和实际单帧分辨率计费。它与推理等级倍率、`complexity`、JPEG 编码压缩质量相互独立。价格设置不会扩大生成能力或解除 GIF 编辑等原有限制。

管理接口的 `image_quality_prices` 是“公开模型 ID → 1K/2K/4K → 质量 → USD/张”的可选对象。创建/更新分组可传该字段；更新时省略字段或传顶层 null 保留原设置，传 `{}` 清空覆盖，格子省略或 null 表示不设覆盖，数字 0 表示免费。质量键用 `xhigh`，不用显示文字 `Extra High`；无效模型、分辨率、质量、负数或非数值会被拒绝。


第 3 节 Python 示例兼容 URL、Base64 和 GIF 保存。若需要只保存 GIF 响应中的 Base64，可用：

```python
import base64
import json
from pathlib import Path

response = json.loads(Path("response.json").read_text())
if response.get("error"):
    raise RuntimeError(response["error"])
for index, item in enumerate(response.get("data", []), 1):
    if "b64_json" not in item:
        raise RuntimeError("本示例只处理 Base64；URL 输出请按第 3 节下载")
    data = base64.b64decode(item["b64_json"], validate=True)
    if data[:6] not in (b"GIF87a", b"GIF89a"):
        raise RuntimeError("返回字节不是 GIF，不应修改后缀冒充动图")
    Path(f"animation-{index}.gif").write_bytes(data)
```

### 8.1 SDK 与前端接入

若 SDK 的参数列表尚未包含 complexity、frame_count、frame_duration_ms，使用它提供的额外 JSON 请求体选项，或直接按 curl 示例发送 HTTP；关键是最终请求体确实含这些 snake_case 字段，而不是只在前端表单里保存。不要改写成 frameCount、frameDurationMs。GIF 展示必须使用原始 GIF 字节或完整图片链接；做缩略图、画布导出或统一 PNG 压缩时应保留原动画供用户下载。客户端遇到新参数不兼容时，先检查最终 HTTP 请求，再排查模型权限与服务端错误。

## 9. 验证范围与使用边界

2026-11-07 通过正式域名补测两款 2.6，共 40 项独立请求：14 个真实输出、26 次符合预期的参数拒绝。每款均取得透明 PNG、不透明 PNG、25/90 两档 JPEG、默认 9 帧 GIF、auto 16 帧 GIF 和固定 896×896 的 2 帧 GIF。已解码验证透明像素、JPEG 编码参数、GIF 的不同画面帧数和逐帧时长；不是只凭 HTTP 200 判断支持。非流式 partial_images=2 只返回完整图片；当时的 WebP、SVG、无效 complexity 与不支持的组合有明确错误。该历史 WebP 拒绝记录保留，WebP 新支持范围以本页新增说明为准。

另有首次测试分组的一次上游认证 401，未计为通过，也未作为“透明参数不支持”的依据；之后使用有近期成功记录的业务分组完成独立样本。测试没有重放失败或未知结果，临时测试权限已清理。上述证据只覆盖已列组合，不代表所有账号、所有尺寸、所有参数组合或高峰吞吐；本节记录此前参数实测；本次 2.6 质量修正与新增 4K 高档位证据见 0.4 节。

2026-11-06 的接入验证使用真实线上 Adobe 账号测试了五款模型的静态生成和 JSON 编辑；2.6 的多图参考、multipart 静态编辑、2K high quality + high complexity 及 GIF 2/4/9/16 帧有成功样本。两款 GIF 的默认 9 帧、auto 16 帧、20/200/2000ms 时长和 25→30ms 取整也有实际输出验证。

2026-11-06 发布后，指定真实账号通过正式域名完成 **18/18** 项验证：五款模型各一项 PNG 生成和 JPEG 编辑、两款 2.6 的 GIF 文生图，以及六项 GIF 编辑/参考图的明确拒绝。验证读取了实际图片文件和 GIF 帧时长，不只判断 HTTP 状态。另两轮普通分组抽样分别为 16/18、13/18，未通过项是上游 408 或提交/上传超时，原记录保留；不能把指定账号成功说成全账号池没有故障。客户遇到此类错误应保留 request_id，并按第 6 节处理。

GIF 参考编辑在本次 Adobe 接入中持续返回上游 500，因此按发布范围明确关闭，并在本地阻止上传和生成。静态编辑、GIF 文生图是不同能力，不能从其中一项成功推断另一项可用。正式发布后的客户入口验证结果记录在 [本期发布记录](https://github.com/tjt740/adobe-relay/blob/main/deploy/primary/adobe-gpt-image-26-gif-20261106.md)。样本通过不代表所有账号权益、所有提示词、所有上游时段或任意高并发都能成功。


> 管理页档位说明（2026-11-11）：新版价格表按 Adobe 官网参数与已核对的页面规则展示。GPT Image 2 在官网无分辨率下拉，管理页仅展示默认尺寸对应的 1K 三档质量；2.5/2.6 为 1K/2K/4K 五档质量。官网没有质量选择的其他模型不生成质量列。此显示调整不删除已有价格，也不改变接口已有尺寸支持和计费规则。官网目录失败时使用有日期的核对快照，自定义模型名称仍可输入。
