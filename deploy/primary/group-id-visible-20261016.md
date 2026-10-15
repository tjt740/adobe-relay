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
