# SimRelay EC20 MVP 实施计划

> **给自动化开发代理的要求：** 按任务逐项执行。每个任务使用测试优先流程：先写失败测试，再写最小实现，再运行验证命令。

**目标：** 构建一个 Go HTTP API 服务，通过配置好的移远 EC20 串口读取和发送支持中文的短信。

**架构：** 使用单个 `simrelay` 二进制，通过命令行参数配置串口和监听地址。串口 AT 传输、EC20 短信能力、HTTP 路由分成独立包，确保没有真实硬件时也能完成核心逻辑测试。

**技术栈：** Go 1.25，`go.bug.st/serial` 串口库，标准库 HTTP 服务和 JSON。短信内容默认按 UCS2 编解码。

---

## 文件结构

- `cmd/simrelay/main.go`：命令行入口，参数解析、依赖组装、HTTP 服务启动。
- `internal/at/client.go`：AT client 接口、命令执行、短信发送提示符处理。
- `internal/at/client_test.go`：fake 串口测试，覆盖命令响应解析和短信发送流程。
- `internal/modem/ec20.go`：EC20 兼容模组操作。
- `internal/modem/ec20_test.go`：fake AT client 测试，覆盖命令顺序和响应解析。
- `internal/httpapi/server.go`：HTTP 处理器和 JSON 错误映射。
- `internal/httpapi/server_test.go`：fake modem 测试，覆盖 API 行为。
- `README.md`：中文优先的安装说明、运行说明、接口示例和 EC20 Linux 注意事项。

## 任务

### 任务 1：AT 传输层

**文件：**

- 新建：`internal/at/client.go`
- 新建：`internal/at/client_test.go`

步骤：

- [ ] 编写失败测试，覆盖普通 AT 命令响应解析和 `OK` 成功判断。
- [ ] 运行 `go test ./internal/at`，确认测试因为实现缺失而失败。
- [ ] 实现最小 AT client，支持带超时的 `Command` 和 `CommandSMS`。
- [ ] 运行 `go test ./internal/at`，确认测试通过。

### 任务 2：EC20 模组能力

**文件：**

- 新建：`internal/modem/ec20.go`
- 新建：`internal/modem/ec20_test.go`

步骤：

- [ ] 编写失败测试，覆盖初始化命令、设备状态解析、中文 UCS2 编解码、短信列表解析、单条短信读取和发送引用号解析。
- [ ] 运行 `go test ./internal/modem`，确认测试因为实现缺失而失败。
- [ ] 基于 AT client 接口实现 EC20 模组方法。
- [ ] 运行 `go test ./internal/modem`，确认测试通过。

### 任务 3：HTTP API

**文件：**

- 新建：`internal/httpapi/server.go`
- 新建：`internal/httpapi/server_test.go`

步骤：

- [ ] 编写失败测试，覆盖 `/healthz`、`/api/v1/device`、短信列表、短信读取、发送参数校验和发送成功。
- [ ] 运行 `go test ./internal/httpapi`，确认测试因为实现缺失而失败。
- [ ] 实现 HTTP handler、JSON 响应和错误码映射。
- [ ] 运行 `go test ./internal/httpapi`，确认测试通过。

### 任务 4：CLI 和串口接入

**文件：**

- 新建：`cmd/simrelay/main.go`
- 修改：`go.mod`
- 新建：`README.md`

步骤：

- [ ] 实现 `--device`、`--baud`、`--listen`、`--timeout` 参数。
- [ ] 使用 `go.bug.st/serial` 打开串口。
- [ ] 启动 HTTP 服务前初始化 EC20 模组。
- [ ] 编写中文 README，包含 EC20 `/dev/ttyUSB*` 使用说明和 API 示例。
- [ ] 运行 `go test ./...` 和 `go build ./cmd/simrelay`。
