# 图片分阶段预留优化

用户要求继续减少内存有余量时的容量 429，保留防 OOM、不新增准入队列，分组 2、7 继续暂停，允许按安全余量调整容量。本轮先保持容器 12 GiB、Go 9 GiB、图片池 7.2 GiB，在同样条件下对照算法改善；不以提高上限代替算法验证。

已知 Content-Length 的请求只为首个至多 1 MiB 读块预留六倍空间，之后仍在每块分配前按实际高水位申请。完整读体后、任何解析和审核前仍保留六倍解码体预算；压缩解码保护及 64 MiB zstd 工作窗口预留保持。没有先分配后申请、强制 GC 或容量等待。

解析审核完成后保留实际输入和每张 1 MiB 工作空间，原生磁盘缓冲使用有界 64 KiB IO；转码、Gemini、中转和磁盘不可用的回退仍须在提交前取得原有三倍输入及每张 32 MiB 输出预算。原下载 URL/任务/出口重试、计费、451、usage、对象存储、联动、告警和账号/用户并发均保留，无迁移及新增配置项。

运行时压力判断只扣除 Go 明确标记为完全空闲、可复用的 heap/free；活跃及未回收对象、碎片、栈和其他 Go 内存继续计入，压力阈值仍约 8.1 GiB，逻辑池仍 7.2 GiB。不强制 GC 或手动归还内存。拒绝日志同时记录已提交、可复用和压力字节，便于分清保留量与实际压力。

对照使用当前线上实际应用 `8db3c34e862dfe07d556ef9b882d0d351113de0f`，同一脱敏轨迹、Go/容器上限、GOMAXPROCS=6、4 CPU 额度及合成输出，增加 600/800 路突发。多数场景使用紧凑合成磁盘预留；另验证正式 160 MiB/张完整预留及低硬限制大输出。它不是生产 Adobe 出图压测或上游吞吐承诺。正式结果在通过验证后归档。

仅发布主服务器 `173.234.15.150` / `https://creative.cheap`，保留原配置、业务数据和其他服务。审计 `/opt/adobe-relay/releases/adobe-memory-staged-20261015/`，回退镜像 `adobe-relay:before-adobe-memory-staged-20261015` 来自发布前实际 8db3c34。原有回退点保留。

只读回退检查 `python3 /opt/adobe-relay/releases/adobe-memory-staged-20261015/adobe-memory-staged-release.py check-rollback`；末尾改为 `rollback` 才恢复代码，保留当前数据、12/9 GiB 配置、7.2 GiB 图片池及暂停状态。依赖同目录 `adobe-memory-burst-release.py`、`image-response-release.py`；存在后续版本/配置时拒绝覆盖，不得绕过保护或恢复旧数据库。
