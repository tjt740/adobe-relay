# Okad–Sub2 可靠联动一期

迭代标识：`okad-sub2-reliability-20261012`。唯一部署目标：`173.234.15.150` / `https://creative.cheap`。

两个仓库均保存以下 Git 标签：

- 发布前 main：`rollback/okad-sub2-reliability-20261012-before-main`
- 本次版本：`release/okad-sub2-reliability-20261012`

发布前 Sub2 main 为 `6bd3dcf5b66d4625cf9fee88638e65950b62d874`；Okad main 为 `b15a8134207131e7817bef79d341112cf09f52d8`。Okad 线上另有已部署但尚未入主干的积分错误、Cookie 推送时间等修复，本次保留；服务器压力监控任务的开发代码不属于本次发布。

## 范围和配置

发布凭据版本与持久发送任务、并发登录互斥、失败补推、代理及人工暂停保护、远端状态核对，以及 Okad 左侧菜单的联动中心。完整时间线和监控中心只讨论过方案，不属于本次实现。

保留线上配置、数据库、账号、代理与回调。可靠联动模式默认关闭；部署代码本身不自动启用 v2、不批量登录或推送账号。原有 v1 Cookie 回调继续使用原配置。需要启用时必须配置两端专用密钥，并验证双方配置，不能只开启 Sub2 一端。

## 快速恢复代码

服务器记录：`/opt/adobe-relay/releases/okad-sub2-reliability-20261012/pre-release.json`。该文件记录两套经过保留的镜像，不含业务凭据。旧镜像不得清理。

恢复**合并前 main 的代码**：

```sh
bash /opt/adobe-relay/deploy/primary/rollback-okad-sub2.sh okad-sub2-reliability-20261012 main
```

恢复**本次发布前实际线上状态**（包含此前独立部署的 Okad 修复）：

```sh
bash /opt/adobe-relay/deploy/primary/rollback-okad-sub2.sh okad-sub2-reliability-20261012 production
```

两种方式都先备份再切换应用镜像，保留当前业务数据；不恢复旧数据库，不覆盖现有环境配置。新增 SQLite 表和兼容字段保留，不做破坏性降级。后续若已启用 v2，回滚会停止该版本的后台队列处理；回滚前应核对仍在验证中的账号，代码回滚不等于撤销业务操作。

本次已验证回退脚本的备份失败阻断、健康检查失败恢复旧代码和保留配置路径。真实生产回退仅在需要时执行，不为测试来回重启线上服务。

Git 主干如需同步回退，应以回滚标签为基准创建恢复提交，正常推送；不要强推或重置共享主干历史。Sub2 的 main 推送会触发自动部署，应先确认恢复提交及部署队列。

## 发布验证

- Okad 后端 433 项通过；Sub2 相关 Go 测试启用 race 检查并使用独立 PostgreSQL。
- 本次发布前补跑账号前端 90 项通过；此前完整相关前端 215 项和类型检查通过。
- 发布脚本执行备份和失败回退的隔离测试。
- 线上只做健康、静态资源、权限及只读业务接口检查；不自动触发 Adobe 登录、验证码或付费模型出图。

部署后的确切提交、镜像与健康结果另存同目录 `deployed.json`，本地保留测试日志及发布记录。
