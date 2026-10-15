# 图片预算 7.2 GiB

用户授权将图片准入预算从 6.75 GiB 调为 7.2 GiB，容器 12 GiB、Go 目标 9 GiB 不变。实现使用 Go 目标的 80%，按整数计算为 7,730,941,133 字节；预算增加约 6.7%，不表示实际吞吐必然同幅增加。

运行时压力保护保持 Go 目标的 90%（约 8.1 GiB），容量不足立即拒绝，不新增队列。输入解析/输出磁盘缓冲、同图下载重试与出口切换、原账号/用户并发、计费、451、usage、联动和告警逻辑均不改。没有数据库迁移或配置项变更。分组 2、7 按用户要求保持暂停，不主动请求 Adobe、登录或发邮件。

验证复用上一轮六组 Linux 隔离测试：64 路完整磁盘预留、400 路 4/8 MiB 输出、3075 条历史时序及 17.5% 原内存路径混合、1.5 GiB 硬限制下 32 路 64 MiB 输出。除特意收紧的 1.5 GiB 场景外均为容器 12 GiB、Go 9 GiB、4 CPU 限额、禁网；本次显式设置 `GOMAXPROCS=6` 与生产一致，计算资源限制仍比生产严格。上一轮测试默认并行度为 4，不能将耗时变化全部归因于预算比例。轨迹含客户重试，尺寸是审计 JSON 字节而非线长，307 条缺失时长采用 180 秒；时间缩放 0.2，输出为合成数据，多数场景用紧凑磁盘预留以适应 CI，不能视为真实 Adobe 吞吐保证。正式结果在通过测试与上线核验后补充。

仅部署主服务器 `173.234.15.150` / `https://creative.cheap`。发布前实际镜像为 `adobe-relay:6868148a26779e94c097eb9626cdd51233a5ac98`。审计目录 `/opt/adobe-relay/releases/adobe-memory-72-20261015/`；回退镜像 `adobe-relay:before-adobe-memory-72-20261015`。此前所有回退点保持。

只读回滚检查：

```sh
python3 /opt/adobe-relay/releases/adobe-memory-72-20261015/adobe-memory-72-release.py check-rollback
```

末尾改为 `rollback` 才恢复应用代码及原 6.75 GiB 预算，保留容器 12 GiB、Go 9 GiB、当前业务数据和暂停状态。脚本依赖同目录 `adobe-memory-burst-release.py`、`image-response-release.py`，检查后续版本与配置，不得绕过检查或恢复旧数据库。
