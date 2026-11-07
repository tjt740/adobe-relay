# Image 2.6 Extra High / Max 与官网名称已上线（2026-11-07）

用户授权推送、合并、主服务器部署，更新接口文档和中英文日志，并在上线后实测两款 2.6 的 Extra High / Max。主站实际应用／镜像为 **`2dc7a2a3ae0761d88b2d7e89964c93b2dc86ff06`**，Sub2API 0.2.19。功能 PR #170，main 合并 `6e3be4943f69536e8dbe1e2168a3d1f0a57c6a5d`；完整构建 [42345092438](https://github.com/tjt740/adobe-relay/actions/runs/42345092438) 成功。合并使用 `[skip ci]`，发布采用受检的精确应用镜像。

## 最终行为

- 对外名称与 Adobe 官网一致：**GPT Image 2.6 Flare** / **GPT Image 2.6 Sunburst**，模型 ID 为 `gpt-image-2.6-flare` / `gpt-image-2.6-sunburst`。旧 `star-swirl` / `aurora` ID 继续兼容，原账号映射和分组白名单仅同变体等价处理，明确映射仍优先；不改存储配置。上游原生 modelVersion 仍为 star-swirl / aurora。
- 两款 2.6 的 `quality=xhigh` 实际提交 `detailLevel=6`，`quality=max` 提交 7，成功响应分别保留 `xhigh` / `max`，不再限制到 High。
- 未知 2.6 质量值在提交前拒绝为 `invalid_quality`；2.6 高档请求被映射到其他模型时拒绝为 `quality_model_not_supported`，不降档生成。auto／省略仍为 medium，旧模型规则保持。`complexity` 仍只接受 low / medium / high。
- 当前请求的实际模型版本和质量档位加入既有脱敏生成日志；不记录提示词、Token、完整请求或签名下载链接。原提交预算、已受理／结果未知不重放、调度和计费规则保持。
- 中英文完整日志按首次接流量的北京时间 11 月 7 日归档，保留同日文档主题、共享登录及其他条目；线上两种语言资源均已核验。

## 上线后正式接口实测

14:49:57 完成四项，使用新模型 ID 经 `https://creative.cheap/v1/images/generations` 调用。四张均完整验证为 **3840×2160 PNG**；逐项关联 HTTP 编号、客户端计费编号、专用测试密钥、账号和生成日志。每项一次外层请求、一次上游提交，零降档、零重放。

| 模型 | 请求／响应质量 | 实际上游档位 | PNG 字节数 |
| --- | --- | --- | --- |
| GPT Image 2.6 Flare | xhigh / xhigh | 6 | 10,171,732 |
| GPT Image 2.6 Flare | max / max | 7 | 10,504,768 |
| GPT Image 2.6 Sunburst | xhigh / xhigh | 6 | 11,038,612 |
| GPT Image 2.6 Sunburst | max / max | 7 | 11,799,698 |

首项已成功出图，但初始验收脚本误将 `usage.request_id` 的 `client:<客户端编号>` 与不同的生成编号比较，导致审计失败。保留 `public/flare-xhigh-result.json`、原图和失败日志，只读关联同一已有请求，补充 `flare-xhigh-reviewed.json`，未重新生成。修正辅助检查后只继续三项未发送的测试；原密钥和续测密钥均经正常 API 清理，临时分组授权还原。四条用量保留，非四次之外再重放首项。

前序调查的六次真实生成独立保留：两次正式接口重现旧版降为 high，四次隔离候选原生调用确认 6 / 7。合计本期质量调查与发布验证十次，零重放；更早两次正式接口 4K High 测试另有记录，不混入本期计数。

## 发布与保护结果

全部时间为北京时间：14:13:13 临时实例接新连接；14:21:01 正式实例接回；14:35:18 临时容器／NAT／滚动租约清理；14:54:26 最终核验通过（[42348841216](https://github.com/tjt740/adobe-relay/actions/runs/42348841216)）。旧侧、临时侧的业务、输入连接及自动检测自然排空，未延后队列或中断已受理任务。临时侧部分长请求遇到上游超时，原事件保留；等待连接和任务占用均稳定归零后才清理。

- 实际切换备份 `/opt/adobe-relay/backups/20261107T062001Z-2dc7a2a3ae0761d88b2d7e89964c93b2dc86ff06` 的 **1060 个账号 ID** 保留，人工暂停、手动代理模式保持。
- 42 个账号的代理变化均有事件链；1 个账号的非来源凭据更新保留实时值，涉及 `_token_version`、`access_token`、`adobe_session_cookie`、`expires_at`，不将其误报为纯 Token 变化。没有恢复旧数据或清零记录。
- 全部受检配置、环境、SMTP、告警、订阅／节点与其他容器保持。迁移集合与校验值一致，无新增迁移；已应用 255 保留，退役 408 门槛为空，1290 保持不存在且零绑定／租约。
- Sub2 保持 54 GiB、Go 43008 MiB、图片 37.8 GiB、GOMAXPROCS=16、CPU shares=4096；Okad 保持 3 GiB、当前 `adobe-team-cookie:native-mail-otp-20261107t041755z`、原容器和配置，未覆盖为历史镜像。
- 两站 HTTPS、实时流、清理后告警心跳、两分钟 13 次被动采样通过，正式实例 0 OOM／重启。管理端模型目录实际返回官网展示名；两种中英文日志资源均读取并验证。无邮件。

## 接口文档

文档／辅助 PR #171，正文源码 **`69a5fcccdf9ee4aa35dad442dd24c05af023d3d2`**，main 合并 `80bd549088e9fbd553d33cbef3f92de01eac4b4c`。两份 guide 完全一致，补充上线后的四项结果、质量对照、拒绝项及示例，共 **23 段**。共享登录、16px 深色正文、绿色导航／表头、代码高亮与主站图标保持；应用鉴权代码和样式未改。

14:56:48 独立更新 [42349123997](https://github.com/tjt740/adobe-relay/actions/runs/42349123997) 成功，仅重启 docs，全部受检现有容器／配置保持。当前 docs 二进制为 `ba6865f5fd014d295538651177e19b8d5dc517f69b76bb62e8df41c1d0ceac66`；原二进制 `d9ac486d20be91e090b805e9dafb72cbb5feaad50d7f9fa687e7c7b2c1d7f3ec` 留存。Markdown SHA256 为 `be750ff277a4067c2e057237bd1306fd33ba6bfb4d28c840c691645721cd4b0e`。

14:58:23 公网 HTTPS 验证 [42349246816](https://github.com/tjt740/adobe-relay/actions/runs/42349246816) 通过：匿名正文拦截、已有 Sub2 管理员登录、阅读、原文下载精确哈希、两款名称／6与7档／23段示例、退出后拦截。凭据只在内存使用；验证没有生成请求。回执按精确源码独立存放，不覆盖旧记录，登录结果未知不重试。

## 检查与边界

- 正式后端、572 项前端、精确镜像及产物门禁全部通过；本地 Adobe 全包及相关质量／映射／白名单／管理目录 race、前端相关 63 项、docs race/vet 通过；原发布保护 44 项及本期最终 36 项通过。
- 首次门禁 [42344693484](https://github.com/tjt740/adobe-relay/actions/runs/42344693484) 因目录测试遗漏两个新名称而拦截，修正测试清单及精确文件哈希后完整重跑通过。原失败保留，没有因该失败改动生产质量逻辑或提前发布。本地直连下载测试曾受进程代理环境影响，移除该测试进程的代理环境后全包通过，生产配置未改。
- 两次浏览器连接均超时，本次只完成实际接口、管理目录及线上资源核验，未完成真实浏览器／手机布局或复制按钮验收；不将既往浏览器记录作为本次证明。
- 高档位实测仅覆盖两款各一次静态 4K 文生图，不作为高档 GIF、图片编辑、全池或高峰可靠性证明。

## 证据和恢复

- 本期服务器目录：`/opt/adobe-relay/releases/adobe-image26-quality-20261107/`，包括原失败、四张原图、关联复核、临时访问清理、最终保护及归档回执。
- 脱敏结果：[public-results.json](adobe-image26-quality-20261107/public-results.json)、[release-verification.json](adobe-image26-quality-20261107/release-verification.json)。完整测试 ID／账号关联另留服务器、Actions 回执和本地审计，提交汇总不含凭据。
- 前序真实测试：`deploy/diagnostics/image26-quality-20261107/verification.json`；官网定义 `official-quality-contract.json`。
- docs 更新审计：`/opt/creative-docs/audit/update-20261107T065648Z/`；公网验证：`/opt/creative-docs/audit/verify-69a5fcccdf9ee4aa35dad442dd24c05af023d3d2/`。
- 回退标签 `adobe-relay:before-adobe-image26-quality-20261107` 对应旧实际 da24f563。真正回退须复核后续版本、配置及在途任务，受控排空，只回代码不回数据；旧代码会重新将 2.6 高档限制到 high。docs 仅按当前版本审查后回独立二进制，不回旧配置或数据库。本次未执行回退。
