# opencode-cleaner

跨平台（macOS / Linux）自动清理 `opencode` 孤儿进程的小工具。  
默认使用“保守策略”：只处理 `PPID=1`、无终端绑定、运行时间超过阈值、且无非本地已建立连接的进程。

## 功能

- `scan`：扫描并输出候选进程
- `clean`：温和清理候选（`SIGTERM`，可选延迟后 `SIGKILL`）
- `service install`：安装自动清理服务
  - macOS：`launchd`
  - Linux：`systemd --user timer`
- `service status / uninstall`：查看状态或卸载

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

## 自动部署（macOS / Linux）

### macOS（launchd）

```bash
./oc-cleaner service install --interval-sec 120 --min-age-min 10
./oc-cleaner service status
```

卸载：

```bash
./oc-cleaner service uninstall
```

### Ubuntu / Linux（systemd user timer）

```bash
./oc-cleaner service install --interval-sec 120 --min-age-min 10
./oc-cleaner service status
```

卸载：

```bash
./oc-cleaner service uninstall
```

> 提示：Linux 使用 `systemd --user`，需确保当前用户会话支持 user services。

## 在其他设备快速部署

可让自动化平台（或大模型代理）按以下步骤执行：

```bash
git clone https://github.com/l2ktech/12-opencode-cleaner.git
cd 12-opencode-cleaner
go build -o oc-cleaner ./cmd/oc-cleaner
./oc-cleaner service install --interval-sec 120 --min-age-min 10
./oc-cleaner service status
```

## 默认安全策略

候选进程需要同时满足：

1. `PPID == 1`
2. `TTY == ?` 或 `??`
3. 运行时长 >= `--min-age-min`
4. 进程没有非 `127.0.0.1 / ::1` 的已建立 TCP 连接

这可最大限度避免误杀正在使用中的前台会话。
