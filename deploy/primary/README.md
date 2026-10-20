# 主服务器发布

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

最新 Sub2 应用：`6941bd6d54de19e4085698fc69147c2fcac88b0c`，上线 Adobe 会话轮换、明确提交前单次 401 恢复及手动额度诊断；保留人工暂停和完整模型验证门槛。额度复测正常，指定账号出图仍返回 408，未验收出图恢复；见 [2026-10-20 会话续期发布记录](adobe-session-renewal-20261020.md)。文档归档不重复部署。

## 每日更新日志

每次应用更新或修复，都应随版本同步维护网站「更新日志」：在 `frontend/src/content/changelog/daily.ts` 添加条目，并维护中英文 `changelog.daily` 文案。按实际发布的北京时间日期（Asia/Shanghai）归入当天，同一天的多次更新合并展示，日期倒序；跨午夜发布要调整日期。简述用户能看到的变化和必要边界，保留历史记录及专题原文，不把未发布或仅测试的功能写成已上线。

内部审计、实际应用 SHA、备份与回退条件仍单独归档在本目录。仅归档发布证据且不改变应用时使用 `[skip ci]`，避免重复部署。
