# 阿里云独立部署

公网入口为 **https://47.106.176.71**（443），也支持 **https://47.106.176.71:6660**。应用后端绑定 **127.0.0.1:6666**，容器内监听 8080。PostgreSQL 和 Redis 仅在独立 Compose 网络内访问，不发布数据库端口。

这套配置用于全新安装，不复制本地数据库、Adobe Cookie 或 API 密钥。线上账号和密钥需重新创建。保留上游源码、许可证及提交历史。

## 初始化与启动

在项目根目录运行：

```sh
python3 deploy/online/init_env.py
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env up -d --build
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env ps
curl --fail http://127.0.0.1:6666/health
```

也可以在构建机上生成服务器架构的镜像，使用 `docker save` / `docker load` 传输，在服务器以 `up -d --no-build` 启动，避免占用小内存服务器的编译资源。

初始化脚本只在 `.env` 不存在时生成独立随机密码和固定加密密钥；文件权限为 0600，重复运行不会重置凭据。管理员邮箱默认为 `admin@sub2api.local`，初始密码见服务器的 `deploy/online/.env` 中 `ADMIN_PASSWORD`。

`.env`、运行数据、日志和镜像归档均不应提交到 GitHub。正式发布通过 GitHub Actions 在推送 `main` 时自动更新国内、国外两台服务器，参见 [Clash 与双机发布说明](../clash/README.md)。本文的 IP 与 Nginx 模板对应国内服务器；已有站点发布时保留各服务器现有的 `.env`、Nginx 和证书续期配置。

## HTTPS / IP 访问

- 网页：`https://47.106.176.71`
- API Base URL：`https://47.106.176.71/v1`
- 兼容原网页端口：`https://47.106.176.71:6660`；HTTP 6660 自动以 308 跳转到 HTTPS 443。
- `6666` 仅监听本机，外部客户端应更新 Base URL。其他项目的端口不变。
- Nginx 使用 `nginx.conf`，保留流式响应及 WebSocket 转发。

使用 Let's Encrypt IP 证书，证书名为 `sub2api-ip`，位于 `/etc/letsencrypt/live/sub2api-ip/`。IP 证书有效期约六天，`sub2api-cert-renew.timer` 每六小时检查续期，续期命令成功后校验并重新加载 Nginx。

首次安装（服务器需 Python 3.11）：

```sh
sudo python3.11 -m venv /opt/sub2api-certbot
sudo /opt/sub2api-certbot/bin/pip install --index-url https://pypi.org/simple certbot==5.4.0
sudo /opt/sub2api-certbot/bin/certbot certonly --non-interactive --agree-tos \
  --register-unsafely-without-email --preferred-profile shortlived \
  --webroot -w /var/www/letsencrypt --ip-address 47.106.176.71 --cert-name sub2api-ip
```

签发前必须让该 IP 的 80 端口默认虚拟主机提供 `/.well-known/acme-challenge/`，静态目录为 `/var/www/letsencrypt`。当前入口在 `/etc/nginx/conf.d/card-recharge.conf` 中，仅新增验证路径，原项目路由保持不变。80 端口需要保持公网可达，以便自动续期。443、6660 也需要允许公网访问。

将 `renew-ip-certificate.sh` 安装为 `/usr/local/sbin/sub2api-cert-renew`（0755），将同目录的 service/timer 安装到 `/etc/systemd/system/`：

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now sub2api-cert-renew.timer
sudo /usr/local/sbin/sub2api-cert-renew --dry-run
sudo systemctl list-timers sub2api-cert-renew.timer
sudo journalctl -u sub2api-cert-renew.service --no-pager -n 30
```

切换前 Nginx 配置备份位于服务器 `/home/admin/sub2api-https-backup/`。应用 `.env` 中的 `BIND_HOST` 应设为 `127.0.0.1`，并通过 Compose 重新创建 app 容器生效。

## 维护

### 短账号登录

登录页支持省略内部邮箱的 `@sub2api.local` 后缀，例如 `pink@sub2api.local` 可直接输入 `pink`。完整邮箱仍可登录。登录页自动补全内部邮箱后使用原有认证接口，不修改数据库邮箱、密码、权限或双重验证；其他邮箱域名的账号仍需输入完整邮箱。直接调用 `/api/v1/auth/login` 的客户端仍应传完整邮箱。

```sh
# 日志
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env logs --tail=100 app
# 停止，保留目录中的数据
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env down
# 已加载新镜像后启动；使用固定版本镜像时需同步修改 .env 中 SUB2API_IMAGE
docker compose -f deploy/online/compose.yml --env-file deploy/online/.env up -d --no-build
```

持久数据位于此目录的 `data/`、`postgres_data/`、`redis_data/`。备份时应包含 `.env` 和应用配置；数据库应使用 `pg_dump`，或停服后复制完整数据目录。恢复已有安装时保留原 JWT/TOTP 密钥和数据库密码。

默认应用内存上限 384MiB、PostgreSQL 192MiB、Redis 96MiB，并降低连接池大小，适用于小规模试用；扩大并发前应根据实际负载调整资源配置。

## 管理员查看全站 API 密钥

- 后台新增「全站 API 密钥」，可按密钥名称、用户名或邮箱搜索，按状态筛选，并分页查看。原有「用户管理 → API 密钥」也提供查看和复制按钮。
- 系统设置中开启 TOTP 功能（部署必须设置固定的 `TOTP_ENCRYPTION_KEY`），各管理员在「个人资料 → 两步验证」用自己的身份验证器绑定。此开关仅开放绑定功能，不会替用户绑定或改变登录密码。
- 完整密钥只能经 `POST /api/v1/admin/api-keys/:id/reveal` 按需获取，传入 `purpose: view` 或 `copy`。此接口始终要求真人管理员的 TOTP 会话验证，不受可选 `step_up_enabled` 开关影响。现有验证授权有效期为 15 分钟；管理 API Key 不可调用。
- 每次返回完整密钥前同步写入 `admin.api_keys.view` 或 `admin.api_keys.copy` 审计记录，包含操作者、目标密钥 ID、所属用户 ID、时间和来源 IP。复制事件表示服务器批准获取密钥用于复制，不代表浏览器剪贴板写入一定成功。日志不包含密钥正文。审计存储不可用时拒绝披露。
- 全站、用户、分组、使用记录等常规 DTO 返回脱敏密钥；普通用户自己的密钥管理接口仍需所有权校验。已有数据库备份、服务器访问等超管能力不在此功能的权限边界内。
- 查看后的明文仅保留在当前组件内，30 秒后、离开页面或切换标签时隐藏；每次点击复制重新请求审计接口。已复制到用户剪贴板的内容不会被自动删除。
