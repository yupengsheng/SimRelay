# SimRelay EC20 MVP 设计文档

## 目标

构建 SimRelay 第一版后端服务：一个 Go 单二进制程序，通过 Linux USB 串口 AT 命令连接移远 EC20 模组，提供 HTTP API 完成短信读取和短信发送。第一版必须优先支持中文短信，因此默认使用 UCS2 文本短信路径。

## 范围

第一版只支持一个通过命令行配置的模组设备。暂不包含 Web 页面、Bot、多设备自动发现、代理能力、eSIM 管理、VoWiFi 等能力。所有项目文档、README、接口说明和面向用户的错误信息都优先使用中文。

## 参考模型

VoHive 的参考价值在于它的运维形态：单二进制、固定服务名、固定安装目录，并通过远程控制入口提供短信能力。SimRelay 第一版只聚焦短信中继能力，不复制 VoHive 的代理、eSIM 和 VoWiFi 功能。

相关资料：

- 移远 EC20-CE Mini PCIe 产品页：<https://www.quectel.com.cn/product/lte-ec20-ce-mini-pcie>
- 3GPP TS 27.005：<https://www.3gpp.org/DynaReport/27005.htm>
- 3GPP TS 27.007：<https://www.3gpp.org/DynaReport/27007.htm>

## 架构

SimRelay 拆成四个边界清晰的模块：

- `cmd/simrelay`：命令行入口，解析参数，打开串口，组装依赖并启动 HTTP 服务。
- `internal/at`：AT 命令传输层，负责按行读取响应、超时控制、错误解析和短信发送时的 `>` 提示符处理。
- `internal/modem`：EC20 兼容短信能力，负责把原始 AT 响应转换成设备状态、短信列表和发送结果。
- `internal/httpapi`：HTTP JSON API，负责请求校验、响应格式和错误码映射。

HTTP 层只依赖一个小的 modem 接口，不直接接触串口。modem 层只依赖 AT client 接口，因此可以用 fake client 测试，不需要真实硬件。

## 命令行配置

```text
simrelay \
  --device /dev/ttyUSB2 \
  --baud 115200 \
  --listen :7575 \
  --timeout 5s
```

参数：

- `--device`：EC20 AT 串口路径，必填。
- `--baud`：串口波特率，默认 `115200`。
- `--listen`：HTTP 监听地址，默认 `:7575`。
- `--timeout`：AT 命令超时时间，默认 `5s`。

## HTTP API

```text
GET  /healthz
GET  /api/v1/device
GET  /api/v1/messages?box=all|inbox|sent|unread
GET  /api/v1/messages/{index}
POST /api/v1/messages
```

发送短信请求：

```json
{
  "to": "+8613800000000",
  "text": "中文测试"
}
```

发送短信响应：

```json
{
  "reference": 42
}
```

## EC20 短信流程

启动时执行初始化：

```text
AT
ATE0
AT+CMGF=1
AT+CSCS="UCS2"
AT+CSMP=17,167,0,8
AT+CPMS="SM","SM","SM"
```

设备状态使用：

```text
AT+CGMI
AT+CGMM
AT+CGSN
AT+CPIN?
AT+CSQ
AT+CREG?
```

短信列表使用数字状态参数 `AT+CMGL=<status>`，避免 `AT+CSCS="UCS2"` 下字符串状态值的字符集歧义。短信读取使用 `AT+CMGR=<index>`。短信发送使用 `AT+CMGS="<UCS2 编码号码>"`，等待 `>` 提示符后写入 UCS2 十六进制短信内容和 Ctrl-Z，最后解析 `+CMGS: <reference>`。

## 错误处理

AT 层区分超时、串口读写失败、命令返回错误和响应格式错误。HTTP 层映射为：

- `400`：请求参数非法。
- `502`：模组命令失败或响应无法解析。
- `504`：模组响应超时。
- `500`：服务内部未知错误。

## 测试

单元测试覆盖 AT 响应解析、EC20 初始化命令、中文 UCS2 编解码、短信发送提示符流程、HTTP 请求校验和 HTTP 错误映射。第一版不自动化真实硬件测试，EC20 实机验证步骤写入 README。
