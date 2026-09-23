# Clash 订阅与节点代理

IP 管理中的「Clash 订阅」支持预览、选择节点导入、刷新和更换订阅，以及测试已导入节点。支持包含 `proxies` 的 Clash YAML（SS、SSR、VMess、VLESS、Trojan、Hysteria、Hysteria2、TUIC、HTTP、SOCKS5、AnyTLS）。不支持只有 `proxy-providers` 的配置、Base64 URI 列表、依赖外部策略组的链式节点或本地证书文件。导入只读取出站节点，不执行订阅中的规则、监听器、脚本或远程配置。

节点经独立的 [Mihomo](https://github.com/MetaCubeX/mihomo/releases/tag/v1.19.31) 服务运行，每个节点有固定的 HTTP/SOCKS 入口及独立随机凭据。分配到账号的代理协议显示为 HTTP，这是应用到 Mihomo 的入口；出口仍使用订阅节点的实际协议。

## 国内、国外同时发布

以后所有正式发布都以 `main` 为准，由 `.github/workflows/deploy-adobe-relay.yml` 同时发布到以下两台服务器：

| 目标 | 网页 | API Base URL |
| --- | --- | --- |
| 国内 | https://47.106.176.71:6660 | https://47.106.176.71:6660/v1 |
| 国外（马尼拉） | https://8.212.129.43 | https://8.212.129.43/v1 |

两台服务器目录均为 `/home/admin/sub2api-online`，Compose 项目均为 `sub2api-online`，数据库、账号、密钥及订阅独立保存。只发布程序，不同步用户数据。保留各服务器现有 `deploy/online/compose.yml`、`.env` 与 Nginx HTTPS 配置；旧 `deploy/adobe-relay` 9500 端口流程不用于当前站点。

GitHub Actions 在推送 `main` 时自动触发，也可在 Actions → Deploy Adobe Relay 选择 main 手动运行。先运行相关测试、构建一个 amd64 应用镜像，将固定版本 Mihomo 一起打包，再分别部署两台服务器。两边独立报告结果，一台失败不会取消另一台。

仓库 Secrets：`DOMESTIC_SSH_KEY`、`DOMESTIC_KNOWN_HOSTS`、`OVERSEAS_SSH_KEY`、`OVERSEAS_KNOWN_HOSTS`。使用各服务器专属的 CI SSH 密钥和已核验的主机公钥，不关闭主机验证。SSH 用户为 `admin`，使用现有免密 sudo 执行 Docker 和发布脚本。

应急手动发布：上传 `deploy/clash/compose.yml`、`init_env.py`、`release-online.sh` 后，在服务器加载新应用镜像及 `metacubex/mihomo:v1.19.31` 镜像，再执行：

```sh
cd /home/admin/sub2api-online
sudo -n bash deploy/clash/release-online.sh adobe-relay:COMMIT
```

脚本先备份并验证数据库、保存应用数据和原环境文件，再只补充缺失的 Clash 控制密钥，更新应用和独立 Mihomo。备份位于权限受限的 `deploy/clash/backups.local/`。应用或运行服务健康校验失败时回退应用镜像，数据库迁移不自动回退。后续重启或更新必须同时使用下面两个 Compose 文件，避免丢失 Clash 配置。

## Docker Compose（现有 online 部署）

在项目根目录操作。先更新应用到包含本功能的版本，备份数据库；迁移 `240_clash_subscription.sql` 随应用启动自动执行。为现有 `.env` **新增**随机控制密钥，不要覆盖其他变量：

```sh
python3 - <<'PY'
from pathlib import Path
import secrets
p = Path('deploy/online/.env')
s = p.read_text()
if not any(line.startswith('CLASH_CONTROLLER_SECRET=') for line in s.splitlines()):
    with p.open('a') as f:
        f.write('\nCLASH_CONTROLLER_SECRET=' + secrets.token_hex(32) + '\n')
p.chmod(0o600)
PY

docker compose --env-file deploy/online/.env \
  -f deploy/online/compose.yml -f deploy/clash/compose.yml up -d --build
```

此 overlay 使用 `app` 服务名；其他部署文件如使用 `sub2api`，需将 overlay 中的 `app` 改成对应名称并连接同一个 Docker 网络。后续启动、更新、查看日志也使用这两个 `-f` 参数。控制端口和节点端口不映射到公网，不需要新增防火墙开放端口。额外预留约 256 MiB 内存给 Mihomo。

## 非 Docker 部署

启动一份专用于本系统的 Mihomo（不要复用个人 Clash 配置，会由系统完整管理），初始化配置包含：

```yaml
external-controller: 127.0.0.1:9090
secret: "替换为随机十六进制密钥"
mode: rule
rules: ["MATCH,REJECT"]
```

在应用进程配置以下环境变量并重启：

| 变量 | 用途 |
| --- | --- |
| `CLASH_CONTROLLER_URL` | 后端访问控制接口的地址，例如 `http://127.0.0.1:9090` |
| `CLASH_CONTROLLER_SECRET` | 与 Mihomo 的 secret 相同 |
| `CLASH_PROXY_HOST` | 后端可访问节点入口的主机，例如 `127.0.0.1`（Docker 为 `mihomo`） |
| `CLASH_PORT_START` | 新安装分配端口起点，默认 20000；已分配端口永久保留，最多保留 4096 个不同名称 |
| `CLASH_CONTROLLER_LISTEN` | Mihomo 内部监听地址，默认 `0.0.0.0:9090`；非 Docker 建议 `127.0.0.1:9090` |

仅在本机或私网监听上述端口。生成的节点入口默认监听 `0.0.0.0`，需使用主机防火墙限制来源。不要在导入后修改代理主机或端口起点；迁移主机时需同步数据库和运行配置。

## 使用

1. IP 管理 → Clash 订阅，粘贴订阅链接 → 预览 / 刷新订阅。
2. 选择节点 → 导入所选 / 保存更换。订阅链接在管理员弹窗中默认明文显示，可点击眼睛图标隐藏；重新打开时回填已保存链接。节点列表显示服务器地址、协议、导入状态、代理入口和测试结果，预览和导入时显示加载动画。
3. 点击「测试已导入节点」查看活性和延迟（最多 4 个并行）。也可以使用主列表的连接测试、出口 IP 和质量检测。
4. 在账号编辑的代理选择中搜索 `Clash · 节点名`，保存后后续请求即通过该节点。其他使用原有代理选择器的功能同样可用。
5. 后续可直接刷新已保存的链接，也可留空复用；更换时输入新链接，再预览并保存。

同名节点保留代理编号、入口、认证信息和账号绑定。未选中或从订阅消失的节点停用，旧入口拒绝连接，避免旧账号绑定使用其他出口。重新出现并选中同名节点可恢复原绑定。节点改名视为新节点，需要重新分配。全部取消选择并保存可停用整个订阅。

订阅托管的代理请从订阅窗口修改；普通编辑、删除和批量删除会阻止修改托管入口。普通手动代理不受影响。

数据库保存订阅和节点配置。应用或 Mihomo 重启后，应用每 15 秒检查并恢复已保存配置，无需重新访问订阅网站。刷新订阅是手动操作。新的订阅下载或校验失败不会写入数据库；内核运行配置应用失败会尝试恢复原配置，恢复失败时界面显示服务未就绪并由后台重试。

单套应用部署使用一份专用 Mihomo；多副本应用必须共享同一个控制服务及可访问的代理主机。数据库行锁串行化导入与恢复。订阅返回重复名称、未知协议或非法节点时整次导入失败，避免默默丢失节点。订阅访问限制为公共 HTTP/HTTPS 地址，大小上限 8 MiB、512 个节点。

## 验证

```sh
cd backend
go test ./internal/clash
```

`manager_integration_test.go` 可在临时 PostgreSQL 与 Mihomo 环境验证真实导入、转发、替换、停用和重启恢复，参见测试文件头部的环境变量说明。不要对生产数据库运行测试。

实现依据：[Mihomo 配置 API](https://wiki.metacubex.one/api/)、[固定出站监听器](https://wiki.metacubex.one/config/inbound/listeners/)。
