# Adobe Relay 升级至 v0.2.8

升级日期：2026-09-26。

## 代码来源与分支

- 工作目录：`sub2api/`。
- 新分支：`codex/upgrade-v0.2.8`。
- 升级前：本地 `main`，提交 `1b76b42c6`，源码版本 `0.2.7`。
- 合入来源：[Wei-Shaw/sub2api v0.2.8](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.8)，标签对应提交 `fd80b08c90b55edcad5b00171b53f08721d30da1`。
- 使用合并保留当前分支的定制历史；`backend/cmd/server/VERSION` 更新为 `0.2.8`，供未注入版本号的本地构建使用。
- 本次只更新本地代码，未推送远程、发布镜像或替换正在运行的服务。`adobe-relay/` 和 `adobe-relay-release/` 工作树未切换或修改。

## 主要功能变化

| 范围 | 升级后的变化 |
| --- | --- |
| 模型与协议 | 增加 GPT-6 Sol/Luna、Claude Opus 5.5、Grok 4.7 的相关模型适配；完善 Opus 5.5 自适应思考、工具调用和 Responses 思考签名回传。 |
| 计费 | 渠道和分组可按 reasoning effort 配置倍率，不再局限于 `max`；按照最终发往上游的 effort 计费；改进视频按秒价格展示。 |
| OpenCode Go | 可查询官方滚动、周、月用量窗口，支持自动刷新、手动查询及同 API Key 共享用量状态，账号列表展示相应用量。 |
| Claude Code | 支持自动同步客户端版本号，减少上游客户端版本变化造成的兼容问题。 |
| 简易模式 | 可选择启用 API Key 消费窗口限制，并控制启动时是否创建默认分组；补充对应部署配置。 |
| 备份与日志 | 新增月度归档和独立保留份数；日志保留范围选择得到完善。 |
| 账号与返佣 | 增加 Codex credits、推荐信息展示；支持登记线下提现，并用幂等标识避免重复扣减。 |
| 内容审核与插件 | 增加 TypeSafe 审核引擎配置和引擎元数据；插件宿主可读取受限的账号元数据。 |
| 网关稳定性 | 完善 Gemini 兼容图片模型的 API Key 转发、图片余额不足时的账号冷却与切换、SSE 终止处理、工具 schema 清理和调度行为。 |
| 发布流程 | 改为分目标构建和产物校验，支持 dry run；保留本分支的 pnpm 版本和不自动回写默认分支版本号的行为。 |

完整的上游说明见 [v0.2.8 发布页](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.8)。这里列出的是相对本分支升级前代码的主要变化。

## 本分支兼容性处理

- 保留 Adobe 图片中转、Kiro、代理过期故障切换、Clash 管理、Codex turn ticket、自定义工具和远程压缩功能。
- Adobe Gemini 模型列表先走 Adobe 自身目录，避免进入不相关的 Antigravity 查询流程。
- 将公开图片请求的宽模型校验与原生 OAuth 图片校验分开，保留 Adobe 模型入口，同时执行上游新增的 Gemini 兼容模型仅限 API Key 账号的限制。
- 合并 OpenCode 用量状态持久化与已有 ticket 状态保护，重新生成依赖注入代码。
- 兼容 Opus 5.5 思考签名与本分支自定义工具；补充同时覆盖普通响应和流式响应的回归测试。
- 保留备份页面统一确认弹窗，接入月度归档删除选项；平台配额面板继续涵盖 Adobe、Kiro 等本分支平台。
- 修正合并后测试构造参数、SQL 模拟数据和平台数量；修正已有 Kiro 图片压缩与定价测试样本。删除误恢复的旧密钥审计查询测试，该方法此前已移除；保留“不保留已删除密钥凭证”的现有集成测试。

**本次没有新增 Adobe Seedance 中转。** 现有 Seedance 仍走 Ark 兼容协议与 OpenAI API Key 账号；Adobe 视频协议目录仍未接入该网关路径。

## 数据库变化

新增 3 个 SQL 迁移，由应用的迁移机制执行：

1. `238b_content_moderation_engine_meta.sql`：内容审核日志增加 `engine_meta`。
2. `239_channel_reasoning_effort_multipliers.sql`：渠道及账号统计价格增加 effort 倍率映射，并迁移已有 `max` 倍率和分组 JSON 配置。
3. `240_affiliate_ledger_operation_id.sql`：返佣流水增加 `operation_id` 和唯一索引，用于线下提现幂等处理。

原有分支迁移 `239_fork_platform_constraints_superset.sql` 保留。此次数据库验证使用独立测试容器，未对正在运行的服务数据库执行迁移。

## 验证记录

| 检查 | 结果 |
| --- | --- |
| 前端全量 Vitest | 358 个测试文件、2690 项测试通过。 |
| 前端生产构建、类型与语言键检查 | `pnpm run build` 通过。 |
| 前端 ESLint | `pnpm run lint:check` 通过。 |
| 后端单元测试 | `go test -tags=unit ./...` 通过；含子测试共 22716 个通过事件，22 个按原有条件跳过。 |
| 后端集成测试 | `go test -tags=integration ./...` 通过；仓储测试在独立 PostgreSQL/Redis 容器中运行，并执行数据库迁移。 |
| 带前端资源的后端构建 | Go 1.27.0、`CGO_ENABLED=0`、`-tags embed` 构建通过；`-version` 输出 `Sub2API 0.2.8`。 |
| 后端新增改动静态检查 | golangci-lint 2.13.0，`--new-from-rev=1b76b42c6`：0 issues。 |
| 发布构建工具 | Linux / Python 3.11 容器内 10 项 release matrix 测试通过。 |
| 部署脚本 | Apple Container、Compose 安全/网关/简易模式配置、运行时资源、Caddy 缓存测试通过。 |
| 合并完整性 | 无冲突残留；`git diff --check` 通过。 |

本机默认代理会干扰 Adobe 回环地址测试和 testcontainers 的 Docker 连接；后端最终验证命令清除代理环境变量，使用项目指定的 Go 1.27.0。前端使用 pnpm 9.15.9。

后端全量静态检查未通过，存在原有告警，主要涉及 Adobe、Clash 等既有代码的错误返回值处理、格式和未使用函数；此次以升级前提交为基线的增量静态检查通过。未为本次升级扩大这些既有代码的清理范围。

需要真实第三方凭证或额外环境变量的可选测试按原有条件跳过，未调用真实 Adobe/其他供应商账号验证出图或视频生成。迁移测试验证测试数据库，不代表已对当前运行实例执行升级。
