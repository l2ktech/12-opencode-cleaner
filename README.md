# opencode-cleaner

## Public Portfolio Summary

This repository is a public support entry for AI development workstation hygiene. It provides a conservative cross-platform CLI for scanning and cleaning orphan `opencode` processes, plus watch-mode memory warnings and macOS/Linux service installation paths.

For interviews, use it as a compact example of operational reliability work around AI coding tools: process classification, safe defaults, CLI ergonomics, background service integration, and low-risk automation design.

## Evidence Entry Points

- [`cmd/oc-cleaner`](cmd/oc-cleaner): CLI entry point.
- [`internal`](internal): process scanning, cleaning, watch, and service logic.
- README sections `scan`, `clean`, `watch`, and `service install`: public usage surface.

## Public Boundary

This repository should not include private process dumps, user session contents, prompts, tokens, machine hostnames, internal automation credentials, or production logs. Keep examples generic and preserve the conservative default behavior: scan first, terminate only narrow orphan-process candidates, and avoid interrupting active sessions.

跨平台（macOS / Linux）自动清理 `opencode` 孤儿进程的小工具。  
默认使用“保守策略”：只处理 `PPID=1`、无终端绑定、运行时间超过阈值、且无非本地已建立连接的进程。

## 功能

- `scan`：扫描并输出候选进程
- `clean`：温和清理候选（`SIGTERM`，可选延迟后 `SIGKILL`）
- `watch`：内存阈值告警（仅提醒，不重启进程）
- `service install`：默认安装并启动两套后台服务（clean + watch）
  - macOS：`launchd`
  - Linux：`systemd --user timer`
- `service status / uninstall`：查看状态或卸载

## 设计原则（保持简单稳定）

- 只清理“疑似孤儿且空闲”的 `opencode` 进程，不重启前台正在使用的会话。
- 默认不做“超内存自动重启会话”这类高侵入动作，避免打断正在进行的工作。
- 通过短间隔定时扫描，实现接近实时的后台维护。

## 安装

### 方式 1：从源码构建（推荐）

```bash
git clone https://github.com/l2ktech/12-opencode-cleaner.git
cd 12-opencode-cleaner
go build -o oc-cleaner ./cmd/oc-cleaner
```

### 方式 2：直接运行（开发模式）

```bash
go run ./cmd/oc-cleaner scan --min-age-min 10
```

## 使用示例

### 1）先扫描（不清理）

```bash
./oc-cleaner scan --min-age-min 10
./oc-cleaner scan --min-age-min 10 --json
```

### 2）执行清理

```bash
# 仅 TERM
./oc-cleaner clean --min-age-min 10

# TERM 后 5 秒仍存活则 KILL
./oc-cleaner clean --min-age-min 10 --kill-after-sec 5
```

### 3）内存阈值提醒（仅提醒）

```bash
# 单次检测：超出 1024MB 的进程会输出 WARN
./oc-cleaner watch --warn-mem-mb 1024 --once

# 持续监控：每 30 秒检测一次
./oc-cleaner watch --warn-mem-mb 1024 --interval-sec 30

# JSON 输出（便于接日志系统）
./oc-cleaner watch --warn-mem-mb 1024 --once --json
```

> `watch` 不会发送 TERM/KILL，只做告警输出。

## 自动部署（macOS / Linux）

### macOS（launchd）

```bash
./oc-cleaner service install --interval-sec 120 --min-age-min 10 --watch-interval-sec 30 --watch-warn-mem-mb 1024
./oc-cleaner service status
```

默认会同时启动：

- `com.wzy.oc-cleaner.clean`（后台清理）
- `com.wzy.oc-cleaner.watch`（后台监控告警）

卸载：

```bash
./oc-cleaner service uninstall
```

### Ubuntu / Linux（systemd user timer）

```bash
./oc-cleaner service install --interval-sec 120 --min-age-min 10 --watch-interval-sec 30 --watch-warn-mem-mb 1024
./oc-cleaner service status
```

默认会同时启动：

- `oc-cleaner-clean.timer`（后台清理）
- `oc-cleaner-watch.timer`（后台监控告警）

建议额外开启 lingering（使用户退出登录后仍可运行 user service）：

```bash
sudo loginctl enable-linger "$USER"
loginctl show-user "$USER" | grep Linger
```

查看定时器触发计划：

```bash
systemctl --user list-timers --all | grep oc-cleaner
```

卸载：

```bash
./oc-cleaner service uninstall
```

> 提示：Linux 使用 `systemd --user`。若未开启 lingering，通常需要用户登录会话存在，定时器才会持续运行。

## 在其他设备快速部署

可让自动化平台（或大模型代理）按以下步骤执行：

```bash
git clone https://github.com/l2ktech/12-opencode-cleaner.git
cd 12-opencode-cleaner
go build -o oc-cleaner ./cmd/oc-cleaner
./oc-cleaner service install --interval-sec 120 --min-age-min 10 --watch-interval-sec 30 --watch-warn-mem-mb 1024
./oc-cleaner service status
```

如果你希望“后台持续运行 + 开机后自动恢复”，建议再执行：

```bash
sudo loginctl enable-linger "$USER"
```

然后重启后验证：

```bash
systemctl --user status oc-cleaner-clean.timer --no-pager
systemctl --user status oc-cleaner-watch.timer --no-pager
systemctl --user list-timers --all | grep oc-cleaner
```

## 快速验证 `watch`（v0.2.0）

如果你只想快速确认“内存超阈值仅提醒”功能，直接执行：

```bash
git clone https://github.com/l2ktech/12-opencode-cleaner.git
cd 12-opencode-cleaner
git checkout v0.2.0
go build -o oc-cleaner ./cmd/oc-cleaner
./oc-cleaner watch --warn-mem-mb 1024 --once
```

## 默认安全策略

候选进程需要同时满足：

1. `PPID == 1`
2. `TTY == ?` 或 `??`
3. 运行时长 >= `--min-age-min`
4. 进程没有非 `127.0.0.1 / ::1` 的已建立 TCP 连接

这可最大限度避免误杀正在使用中的前台会话。

## 关于“高内存提醒”

当前版本不自动重启高内存会话进程（避免中断工作流）。推荐做法：

1. 用 `scan` 先观察状态
2. 仅通过 `clean` 回收明确符合条件的孤儿进程
3. 按需调小 `--interval-sec` 以提高巡检频率
