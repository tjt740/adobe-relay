# Adobe GPT Image 2.6 发布记录

日期：2026-11-06（北京时间）

## 结果

主服务器已运行应用提交 `a1cf154c64badf2f889c1850f7785a64eeb324b4`。正式发布工作流 `42085544471` 的后端、前端、amd64 镜像、产物校验、远端发布和 `https://creative.cheap/health` 检查均成功。实际切换备份为：

`/opt/adobe-relay/backups/20261105T184520Z-a1cf154c64badf2f889c1850f7785a64eeb324b4`

发布脚本只替换 `adobe-relay-app-1`，保留 PostgreSQL、Redis、Mihomo、Okad、配置、应用数据和 Nginx。没有数据库迁移，发布前后的业务配置、账号凭据、Cookie、代理、SMTP、告警、订阅和调度规则没有主动修改。

## 代码与模型

PR #139 将两个独立的 Adobe 模型族加入目录：

- `gpt-image-2.6-flare` → `firefly-gpt-image-2-6-flare` → `gpt-image-2.6-star-swirl`
- `gpt-image-2.6-sunburst` → `firefly-gpt-image-2-6-sunburst` → `gpt-image-2.6-aurora`

两者沿用 GPT Image 2.5 的提交、编辑、参考图、任务轮询、下载、计费和错误处理链路。质量映射遵守 2.6 的 detailLevel 1–5；auto／未传质量为 3，low 为 1，high、xhigh、max 不超过 5。显式尺寸校验 480–3840、16 像素倍数、655,360–8,294,400 总像素和 3:1 最大宽高比。非法尺寸在本地拒绝，不静默调整；其他模型不走这组校验。

## 验证

隔离验证和云端门禁：

- 后端 Adobe 目录、请求体、质量、计费和映射回归通过；后端包、服务、处理器、路由和相关 race 测试通过。
- 前端目标 Vitest、locale 完整性、类型检查和生产构建通过；云端 build-only 门禁 `42082974335` 的后端、前端、amd64 镜像和 release artifact 全部成功。
- 主服务器发布后，指定邮箱 `jhmvoq73843@hotmail.com` 匹配到账号 ID 2933，账号发布前状态为 active。账号级 Flare 测试 `42088714236` 和 Sunburst 测试 `42088886266` 均各发送一次 POST，收到 HTTP 200 的 SSE `test_complete=true`；耗时分别为 12.199 秒和 21.628 秒。审计输出只保存邮箱 SHA-256，不保存凭据。
- 公共 API Key id=12、group=7 的 Flare 样本 `42087489745` 和 Sunburst 样本 `42087822865` 各一次在 submit 阶段返回上游 408（未受理、无图片）。两次都留下独占开始标记，结果不明或已受理时没有重放。该 Key 选中的账号／出口与账号 2933 的成功测试不是同一条件，因此不把它们混为一项结果。

账号 2933 的两款模型实测通过是本次指定账号验收依据；公共 Key 的 408 仍保留为真实限制证据，不代表全池或高峰吞吐。未发送测试邮件，未主动推送 Cookie 或登录，未做生产手机布局验收。

## 回退边界

切换备份内的 `previous-image` 记录原应用镜像。标准 `deploy/primary/release.sh` 已验证备份和健康状态；本次没有自动回退。若后续需要回退，先确认账号 2933 的测试记录、两次公共 Key 未受理请求、当前配置和在途任务，再使用发布锁仅恢复 app 镜像；不恢复数据库、账号、凭据、代理、Cookie 或订阅历史。

