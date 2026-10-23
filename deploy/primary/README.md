# 主服务器发布

当前已上线 Adobe 出口复核与 Clash 订阅流量保护，接续保留 408 名额回收、v0.2.17 和订阅管理功能。实际应用、验证与边界见 [本次发布记录](adobe-route-health-20261023.md)。

自 2026-10-11 起，本项目所有正式发布仅更新主服务器 **173.234.15.150**，网站为 **https://creative.cheap**，API 为 `https://creative.cheap/v1`。国内 `47.106.176.71` 和菲律宾 `8.212.129.43` 不再是发布目标。

推送 `main` 后由 `.github/workflows/deploy-adobe-relay.yml` 测试、构建 linux/amd64 镜像并发布到主服务器；也可在 Actions 手动运行 main。发布使用 `PRIMARY_SSH_KEY` 和 `PRIMARY_KNOWN_HOSTS`，SSH 用户为 root，严格校验已有主机公钥。

服务器保留现有 `/opt/adobe-relay/shared/.env`、`compose.yml`、`clash.yml`、`resources.yml`、数据目录及 Nginx 配置。`adobe-relay` 命令使用这些配置，Compose 项目为 `adobe-relay`。Okad 网络及回调配置也必须保留；发布前应将线上功能纳入源码，避免覆盖独立任务部署的修复。

发布脚本 `deploy/primary/release.sh` 使用服务器发布锁，备份并验证 PostgreSQL 转储、保存配置和应用数据，只更新 app 容器。数据库、Redis、Mihomo 和 Okad 容器保持运行。应用健康检查失败时恢复旧镜像；数据库迁移不自动回退。备份保存在 `/opt/adobe-relay/backups/`。

应急手动发布：先加载已验证的应用镜像，再上传并执行同一脚本：

```sh
bash /opt/adobe-relay/deploy/primary/release.sh adobe-relay:COMMIT
curl --fail https://creative.cheap/health
```

旧 `deploy/online` 和 `deploy/adobe-relay` 的 IP 与目录说明仅用于历史维护，不作为默认发布目标。

受控滚动切换时，在已验证新镜像的临时实例接走新连接、旧实例排空后，可调用 `release.sh IMAGE EXPECTED_CURRENT --keep-canary-on-failure`。此模式要求同镜像临时实例健康且滚动租约有效；正式实例启动失败时保留临时实例服务，不自动启动旧版，以免旧版启动清理删除在途请求的并发槽。检查后再重试或安排排空回退。普通发布的原自动回退行为保持。

最新 Sub2 应用：`3d3819b888ba0a10a7b1dde525ccdc7a80ff2383`。正式检查及构建 `39942151619` 成功，2026-10-23 北京时间 19:25:09 正式实例接回、19:29:33 临时资源清理、19:32:52 最终核验通过。订阅流量与来源展示、耗尽保护、最多三来源 408/429 检测复核及生成证据冷却上线；原配置、人工暂停、SMTP、告警、资源和其他服务保留。现有 212 账号保留，15 个代理变化有事件链。一次后续 Fossati GPT Image 2 真实检测在节点 1283 返回 403，未出图、未使用 1290，仍保留原隔离，不能宣称账号或全池恢复；三来源分支有隔离回归覆盖。浏览器工具超时，仅完成线上 API／资源核验。详见 [发布记录](adobe-route-health-20261023.md)。

此前实际应用及历史容量记录：`3d0f78a461e37b8a9bbaea8a807bfede57b2a60e`。PR #100 合并，正式构建 `39761713163` 成功，受控滚动与最终核验完成。北京时间 2026-10-22 18:35 容量快照：693 个未恢复 408 账号释放原绑定，健康空闲名额 0→649，154 个前后均健康的原全 408 节点完全空出；原配置、人工暂停、资源、SMTP、告警和其他服务保持。新流程及中英文每日日志上线；网站接口／资源、两分钟 13 次被动采样及数据核验通过，Chrome 超时未完成真实布局验收。见 [发布记录](proxy-408-quarantine-20261022.md)。原 [订阅管理](clash-subscription-manager-20261022.md)及 [提速日志](release-speed-changelog-20261021.md)保留；文档归档不重复部署。

## 每日更新日志

每次应用更新或修复，都应随版本同步维护网站「更新日志」：在 `frontend/src/content/changelog/daily.ts` 添加条目，并维护中英文 `changelog.daily` 文案。按实际发布的北京时间日期（Asia/Shanghai）归入当天，同一天的多次更新合并展示，日期倒序；跨午夜发布要调整日期。每天有重点概览，每项写清改动原因、具体调整、使用场景和适用边界，并说明验证结果与局限；中英文同步，以 10.17 专题为完整度标准。保留历史记录及专题原文，不把未发布或仅测试的功能写成已上线。

内部审计、实际应用 SHA、备份与回退条件仍单独归档在本目录。仅归档发布证据且不改变应用时使用 `[skip ci]`，避免重复部署。

## 发布准备提速

后端及发布脚本检查、前端检查、amd64 镜像构建在独立 runner 上并行执行。测试命令和范围保持；镜像任务先产出 `candidate-image`，三个任务全部成功后才由 `build` 验证并发布正式 `release-images`，保持旧的成功构建及产物含义。`deploy` 必须等待此门槛通过。手动 `build_only=true` 继续只检查和构建，不连接生产服务器，其独立队列可取消同一分支的旧构建；正式发布队列仍串行且不自动取消。

Go 缓存按操作系统、架构、Go 版本、依赖及提交标识保存，依赖相同的新提交恢复最近一次成功检查的缓存，避免旧的固定依赖缓存命中后一直不更新。每轮先清理旧测试结果缓存，保留依赖及编译缓存，让测试仍在本轮数据库上执行。pnpm 使用锁文件缓存，Docker 为主服务器 amd64 构建使用独立缓存范围。首次冷构建及缓存被清理时仍需要正常下载和编译，提速幅度以 Actions 实际耗时为准。

标准上传步骤先校验产物 SHA-256，再用 `load-image.sh` 将 gzip 原样传到主服务器，在远端解压并加载镜像，避免在 SSH 链路发送解压后的完整 tar。已有 SSH 主机密钥检查保持；解压、加载或连接失败均终止该步骤。手工使用同一入口时也必须先验证归档的 SHA-256：

```sh
sha256sum -c release-images.tar.gz.sha256
bash deploy/primary/load-image.sh release-images.tar.gz root@173.234.15.150 \
  -i /path/to/existing-key -o BatchMode=yes -o StrictHostKeyChecking=yes
```

这些调整只减少发布准备和传输的等待，既有发布锁、实际版本校验、备份、滚动租约、在途任务排空及回退条件继续适用。单次 CI 构建耗时与完整上线耗时分开记录，不能把排空时间当作可以强制跳过的固定等待。网站的每日更新日志仍随实际应用发布按北京时间维护；仅更改发布基础设施不应伪造一次网站应用上线记录。
