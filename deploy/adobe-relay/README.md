# Adobe 中转旧部署（归档）

当前 main 已改为国内、国外两台服务器同步发布，参见 [Clash 部署与发布说明](../clash/README.md)。本文件的 9500 端口及旧目录仅保留为历史记录，不再用于当前发布。

- 仓库：https://github.com/tjt740/adobe-relay （私有）
- 页面：http://47.106.176.71:9500
- API：http://47.106.176.71:9500/v1
- 服务器目录：`/home/admin/adobe-relay`
- Compose 项目：`adobe-relay`，独立 PostgreSQL、Redis 和应用数据。

## 发布

推送 `main` 后由 `.github/workflows/deploy-adobe-relay.yml` 在 GitHub 构建 amd64 镜像，再通过 SSH 上传并启动。也支持手动运行工作流。首次发布在本地构建后通过相同的部署脚本启动。

仓库 Secrets：`ALIYUN_HOST`、`ALIYUN_USER`、`ALIYUN_SSH_KEY`、`ALIYUN_KNOWN_HOSTS`。使用已核验的主机公钥，不跳过 SSH 主机验证。上游工作流保留，但在此独立仓库中禁用。

```sh
# 服务器端，首次执行生成随机密码，不覆盖已有配置
python3 deploy/adobe-relay/init_env.py
# 先 docker load 加载相应镜像，再执行
bash deploy/adobe-relay/release.sh adobe-relay:COMMIT
```

管理员邮箱为 `admin@sub2api.local`（登录页也可输入 `admin`），随机初始密码在服务器 `deploy/adobe-relay/.env` 的 `ADMIN_PASSWORD` 中。全新安装不复制旧服务用户、密钥或账号。

部署配置和数据不提交 Git。备份必须包含 `.env`、应用数据和 PostgreSQL 的 `pg_dump`；恢复时保留原加密密钥。

```sh
docker compose -f deploy/adobe-relay/compose.yml --env-file deploy/adobe-relay/.env ps
docker compose -f deploy/adobe-relay/compose.yml --env-file deploy/adobe-relay/.env logs --tail=100 app
curl --fail http://127.0.0.1:9500/health
```

端口 9500 需在安全组和主机防火墙中放行。后续可使用 HTTPS 域名反代至 9500。重复部署保留所有数据，失败时部署脚本尝试回退到之前的应用镜像；数据库迁移不自动回退。
