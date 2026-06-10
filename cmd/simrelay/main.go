package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.bug.st/serial"

	"github.com/yupengsheng/SimRelay/internal/at"
	"github.com/yupengsheng/SimRelay/internal/httpapi"
	"github.com/yupengsheng/SimRelay/internal/modem"
)

func main() {
	cfg := parseConfig()
	if cfg.device == "" {
		log.Fatal("必须通过 --device 或 SIMRELAY_DEVICE 指定 EC20 AT 串口，例如 /dev/ttyUSB2")
	}

	port, err := serial.Open(cfg.device, &serial.Mode{BaudRate: cfg.baud})
	if err != nil {
		log.Fatalf("打开串口失败: %v", err)
	}
	defer func() {
		if err := port.Close(); err != nil {
			log.Printf("关闭串口失败: %v", err)
		}
	}()

	ec20 := modem.NewEC20(at.NewClient(port, cfg.timeout))
	if err := ec20.Init(); err != nil {
		log.Fatalf("初始化 EC20 失败: %v", err)
	}
	var device httpapi.Modem = ec20
	if cfg.qmiDevice != "" {
		device = modem.NewQMIEnhanced(ec20, modem.QMIConfig{
			Device:           cfg.qmiDevice,
			Command:          cfg.qmiCommand,
			NetworkInterface: cfg.qmiInterface,
			Timeout:          cfg.qmiTimeout,
			CacheTTL:         cfg.qmiCacheTTL,
			UseProxy:         cfg.qmiProxy,
		})
		log.Printf("QMI 状态增强已启用，控制设备 %s，网卡 %s", cfg.qmiDevice, cfg.qmiInterface)
	}

	server := &http.Server{
		Addr:              cfg.listen,
		Handler:           httpapi.New(device),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("SimRelay 正在监听 %s，设备 %s，波特率 %d", cfg.listen, cfg.device, cfg.baud)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}

type config struct {
	device       string
	baud         int
	listen       string
	timeout      time.Duration
	qmiDevice    string
	qmiCommand   string
	qmiInterface string
	qmiTimeout   time.Duration
	qmiCacheTTL  time.Duration
	qmiProxy     bool
}

func parseConfig() config {
	defaults := config{
		device:       os.Getenv("SIMRELAY_DEVICE"),
		baud:         envInt("SIMRELAY_BAUD", 115200),
		listen:       envString("SIMRELAY_LISTEN", ":7575"),
		timeout:      envDuration("SIMRELAY_TIMEOUT", 5*time.Second),
		qmiDevice:    os.Getenv("SIMRELAY_QMI_DEVICE"),
		qmiCommand:   envString("SIMRELAY_QMI_COMMAND", "qmicli"),
		qmiInterface: os.Getenv("SIMRELAY_NET_INTERFACE"),
		qmiTimeout:   envDuration("SIMRELAY_QMI_TIMEOUT", time.Second),
		qmiCacheTTL:  envDuration("SIMRELAY_QMI_CACHE_TTL", 15*time.Second),
		qmiProxy:     envBool("SIMRELAY_QMI_PROXY", false),
	}

	flag.StringVar(&defaults.device, "device", defaults.device, "EC20 AT 串口路径，例如 /dev/ttyUSB2")
	flag.IntVar(&defaults.baud, "baud", defaults.baud, "串口波特率")
	flag.StringVar(&defaults.listen, "listen", defaults.listen, "HTTP 监听地址")
	flag.DurationVar(&defaults.timeout, "timeout", defaults.timeout, "AT 命令超时时间")
	flag.StringVar(&defaults.qmiDevice, "qmi-device", defaults.qmiDevice, "可选，QMI 控制设备路径，例如 /dev/cdc-wdm0")
	flag.StringVar(&defaults.qmiCommand, "qmi-command", defaults.qmiCommand, "qmicli 命令路径")
	flag.StringVar(&defaults.qmiInterface, "net-interface", defaults.qmiInterface, "可选，蜂窝网卡名，例如 wwp0s26f7u1i4")
	flag.DurationVar(&defaults.qmiTimeout, "qmi-timeout", defaults.qmiTimeout, "单条 qmicli 查询超时时间")
	flag.DurationVar(&defaults.qmiCacheTTL, "qmi-cache-ttl", defaults.qmiCacheTTL, "QMI 状态缓存时间")
	flag.BoolVar(&defaults.qmiProxy, "qmi-proxy", defaults.qmiProxy, "qmicli 是否使用 --device-open-proxy")
	flag.Parse()

	return defaults
}

func envBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		log.Fatalf("%s 必须是布尔值，例如 true 或 false: %v", key, err)
	}
	return value
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		log.Fatalf("%s 必须是整数: %v", key, err)
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		log.Fatalf("%s 必须是 duration，例如 5s: %v", key, err)
	}
	return value
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "SimRelay EC20 短信中继服务\n\n")
	flag.PrintDefaults()
}

func init() {
	flag.Usage = usage
}
