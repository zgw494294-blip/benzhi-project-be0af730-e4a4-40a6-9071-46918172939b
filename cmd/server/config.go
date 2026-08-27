package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

type config struct {
	addr            string
	dataDir         string
	selfcheck       bool
	shutdownTimeout time.Duration
}

func parseConfig() (config, error) {
	defaultAddr := "127.0.0.1:19081"
	if port := os.Getenv("PORT"); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return config{}, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
		}
		defaultAddr = net.JoinHostPort("127.0.0.1", strconv.Itoa(n))
	}
	cfg := config{}
	flag.StringVar(&cfg.addr, "addr", defaultAddr, "HTTP 监听地址")
	flag.StringVar(&cfg.dataDir, "data", "./data", "本地数据目录")
	flag.BoolVar(&cfg.selfcheck, "selfcheck", false, "执行真实 HTTP 全流程自检后退出")
	flag.DurationVar(&cfg.shutdownTimeout, "shutdown-timeout", 5*time.Second, "优雅关闭超时")
	flag.Parse()
	if _, _, err := net.SplitHostPort(cfg.addr); err != nil {
		return config{}, fmt.Errorf("-addr 格式无效: %w", err)
	}
	if cfg.shutdownTimeout <= 0 {
		return config{}, fmt.Errorf("-shutdown-timeout 必须大于零")
	}
	return cfg, nil
}
