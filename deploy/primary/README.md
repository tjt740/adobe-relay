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
