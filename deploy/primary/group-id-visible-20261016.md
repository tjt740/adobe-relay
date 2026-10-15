# 分组 ID 默认显示

标识 `group-id-visible-20261016`。用户授权推送并部署到主服务器 `173.234.15.150` / `https://creative.cheap`。

分组管理默认显示名称右侧的 ID 列。列偏好版本从 2 升到 3：新浏览器直接显示 ID；已有浏览器首次加载新版时清除旧的 ID 隐藏选择，保留其他列偏好。升级完成后，管理员仍可通过「列设置」自行隐藏或显示 ID，后续刷新保留新选择。简易模式沿用相同默认值。

只调整前端默认值及其兼容逻辑，不改分组编号、名称、排序、账号调度、代理、Cookie、内存配置、SMTP 或告警。分组 2、7 的暂停状态保持。无需数据库迁移，不发布 Okad。

本地 20 项分组页面回归通过，覆盖新浏览器默认值、无版本及版本 2 的旧偏好迁移、其他隐藏列保留、升级后新选择持久化，以及复制与绑定界面。正式工作流增加同组回归，继续执行原有发布检查与构建。

发布前实际应用为 `c138d73dc67b358a72ffcc8e09f5d62ff8d791dc`。服务器审计目录为 `/opt/adobe-relay/releases/group-id-visible-20261016/`；保留回退镜像 `adobe-relay:before-group-id-visible-20261016`。正式发布脚本在切换前验证数据库备份并保留现有配置。

只读回退检查：

```sh
python3 /opt/adobe-relay/releases/group-id-visible-20261016/group-id-visible-release.py check-rollback
```

末尾改为 `rollback` 才恢复应用代码；依赖同目录 `image-response-release.py` 和 `expected-sha`。先校验后续版本、配置和其他容器，不绕过保护，不恢复旧数据库。浏览器已升级的列选择保存在各浏览器本地，代码回滚不主动重写它们。

## 实际发布与核验

功能 PR [#72](https://github.com/tjt740/adobe-relay/pull/72) 已合并，实际应用为 `c574dcc0dc6369cd05d94b25f75d4ea66c1e4bb1`。正式工作流 [38768162942](https://github.com/tjt740/adobe-relay/actions/runs/38768162942) 成功，北京时间 2026-10-16 00:37:39 启动新容器。后端检查、29 项发布保护、357 项前端回归、3 项 i18n、TypeScript 和镜像构建通过。前后 backend Git tree 均为 `e59cc6f3f80e38d607f5878e78992cee99c8f973`，没有后端行为改动。

只读核验工作流 [38769850399](https://github.com/tjt740/adobe-relay/actions/runs/38769850399) 成功。两站 HTTPS、实时状态流、报表策略和新进程告警心跳正常；原配置、SMTP、告警规则、Okad 设置和其他容器身份/镜像一致。保留容器 12 GiB、Go 9216MiB、GOMAXPROCS=6 和原内存算法；本次未扩容。Okad 仍是 `b0f8e233eaac40d2c88119eb350b87fc536908fe` 及已上线叠加功能。

实际切换备份 `/opt/adobe-relay/backups/20261015T163729Z-c574dcc0dc6369cd05d94b25f75d4ea66c1e4bb1` 与核验时的 956 个账号 ID 完全一致，人工暂停保持。#198/#328/#254 的代理变化都有原自动切换记录；#719/#720/#721/#1014/#1015/#1018 的 Cookie/Token 更新均对应 00:37–00:38 原 Okad credentials 接口成功请求。#719 状态/调度同时变化；#1018 当时已有检测显示 interrupted。核验不主动出图、检测、登录或发邮件，不恢复旧业务数据。分组 2 的 191 个账号和分组 7 的 18 个账号均继续不可调度。

线上分组页资源从 `GroupsView-B7C8Pnif.js` 更新为 `GroupsView-D1p-GHgW.js`，确认列偏好版本为 3 且含 ID 显示迁移。实际浏览器在旧版先隐藏 ID，加载新版后不点击 ID 开关即自动显示；新版截图已保存。新浏览器默认值由页面回归覆盖。

发布标签 `release/group-id-visible-20261016` 指向实际应用，前主干标签 `rollback/group-id-visible-20261016-before-main` 指向 `b589d78184178d41ea04fc6846068ad9b1b0fd95`。代码回滚保护检查通过，回退镜像保留为原实际 `c138d73dc`。审计目录含 before/after、business/proxy/account-state、credential-update-review、final-verification、expected-sha；本地另保留前后浏览器截图与线上资源摘要。

归档提交使用 `[skip ci]`，不重复部署或重启。辅助核验分支 `codex/verify-group-id-visible` 只用于只读审计，不将其工作流替换合入主干；后续核验仍须填写实际应用 SHA。
