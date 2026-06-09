# SimRelay

SimRelay 是一个短信收发后端服务，第一版优先支持移远 EC20 模组。它通过 Linux USB 串口发送 AT 命令，把 SIM 卡短信能力封装成 HTTP API，适合在本地设备、树莓派、NAS 或小型 Linux 主机上运行。

## 当前能力

- 支持单个移远 EC20 设备。
- 支持中文短信发送和读取，默认使用 UCS2 文本短信模式。
- 提供 HTTP API 查询设备状态、读取短信列表、读取单条短信、发送短信。
- 提供内置 Web 控制台，可在浏览器里刷新设备状态、筛选短信、查看详情和发送短信。
- 通过命令行参数配置串口、波特率、HTTP 监听地址和 AT 超时时间。
- 支持 Docker / Docker Compose 部署。

暂不包含 Bot、多设备管理、代理、eSIM、VoWiFi 等能力。

## 环境要求

- Linux 主机。
- 移远 EC20 模组和可用 SIM 卡。
- EC20 已通过 USB 接入系统，并能看到 `/dev/ttyUSB*` 设备。
- Go 1.25 或更新版本用于本地构建。

EC20 常见会暴露多个 `/dev/ttyUSB*` 设备。AT 命令口通常需要按实际系统确认，常见示例是 `/dev/ttyUSB2`。如果串口被 `ModemManager` 占用，直接发送 AT 命令可能失败，可以先停止它：

```bash
sudo systemctl stop ModemManager
```

## 构建

```bash
go build -o simrelay ./cmd/simrelay
```

## 运行

```bash
./simrelay \
  --device /dev/ttyUSB2 \
  --baud 115200 \
  --listen :7575 \
  --timeout 5s
```

参数说明：

- `--device`：EC20 AT 串口路径，必填。
- `--baud`：串口波特率，默认 `115200`。
- `--listen`：HTTP 监听地址，默认 `:7575`。
- `--timeout`：AT 命令超时时间，默认 `5s`。

也可以使用环境变量，便于容器部署：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `SIMRELAY_DEVICE` | 空 | EC20 AT 串口路径 |
| `SIMRELAY_BAUD` | `115200` | 串口波特率 |
| `SIMRELAY_LISTEN` | `:7575` | HTTP 监听地址 |
| `SIMRELAY_TIMEOUT` | `5s` | AT 命令超时时间 |
| `SIMRELAY_ADMIN_USERNAME` | `admin` | Web 控制台账号 |
| `SIMRELAY_ADMIN_PASSWORD` | `admin` | Web 控制台密码，部署时建议通过环境变量覆盖 |

## Web 控制台

服务启动后访问：

```text
http://127.0.0.1:7575/
```

控制台提供：

- 设备摘要：SIM 状态、网络注册状态、RSSI、短信数量。
- 短信列表：支持全部、未读、收件箱、已发送筛选。
- 短信详情：点击列表项目查看完整内容。
- 发送短信：填写号码和内容后调用后端 API 发送。
- 深浅色切换：偏后台控制台风格，参考 VoHive 的紧凑交互布局。

## Docker 部署

本仓库提供 `Dockerfile` 和 `docker-compose.yml`。EC20 AT 串口必须映射进容器。当前 Docker VM 实机验证可用的 AT 口是 `/dev/ttyUSB3`，宿主机端口使用 `7576`，避免和现有 VoHive 的 `7575` 冲突。

```bash
docker compose up -d --build
```

如果 Docker VM 使用 `/opt/stacks` 管理服务，可以放到类似目录：

```bash
sudo mkdir -p /opt/stacks/simrelay
sudo cp docker-compose.yml /opt/stacks/simrelay/compose.yml
cd /opt/stacks/simrelay
docker compose up -d
```

部署前确认：

```bash
ls -l /dev/ttyUSB*
sudo systemctl stop ModemManager
```

如果串口不是 `/dev/ttyUSB3`，同步修改 `docker-compose.yml` 中的 `SIMRELAY_DEVICE` 和 `devices` 映射。如果宿主机没有其他服务占用 `7575`，也可以把端口映射改回 `7575:7575`。

## HTTP API

### 健康检查

```bash
curl http://127.0.0.1:7575/healthz
```

### 查询设备状态

```bash
curl http://127.0.0.1:7575/api/v1/device
```

响应示例：

```json
{
  "manufacturer": "Quectel",
  "model": "EC20",
  "imei": "867698040000001",
  "sim": "READY",
  "signal_rssi": 18,
  "signal_ber": 99,
  "registered": true
}
```

### 读取短信列表

```bash
curl 'http://127.0.0.1:7575/api/v1/messages?box=all'
```

`box` 可选值：

- `all`：全部短信。
- `inbox`：已读收件箱短信。
- `unread`：未读短信。
- `sent`：已发送短信。

### 读取单条短信

```bash
curl http://127.0.0.1:7575/api/v1/messages/1
```

### 发送中文短信

```bash
curl -X POST http://127.0.0.1:7575/api/v1/messages \
  -H 'Content-Type: application/json' \
  -d '{"to":"+8613800000000","text":"中文测试"}'
```

响应示例：

```json
{
  "reference": 42
}
```

## EC20 初始化行为

服务启动后会执行以下 AT 初始化命令：

```text
AT
ATE0
AT+CMGF=1
AT+CSCS="UCS2"
AT+CSMP=17,167,0,8
AT+CPMS="SM","SM","SM"
```

其中 `AT+CSCS="UCS2"` 和 `AT+CSMP=17,167,0,8` 用于优先支持中文短信。

## 开发验证

```bash
go test ./...
go build ./cmd/simrelay
docker build -t simrelay:latest .
```

## 参考资料

- VoHive 发布仓库：`git@github.com:iniwex5/vohive-release.git`
- 移远 EC20-CE Mini PCIe 产品页：<https://www.quectel.com.cn/product/lte-ec20-ce-mini-pcie>
- 3GPP TS 27.005 短信 AT 命令标准：<https://www.3gpp.org/DynaReport/27005.htm>
- 3GPP TS 27.007 设备状态、SIM 状态和网络注册 AT 命令标准：<https://www.3gpp.org/DynaReport/27007.htm>
