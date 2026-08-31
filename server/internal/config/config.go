package config

import "os"

// Config 是进程启动所需的运行参数。S01 只用到 Addr，其它字段留给后续步骤。
type Config struct {
	Addr string
}

// Load 从环境变量读取配置。ADDR 为空时默认监听 :8080。
func Load() (Config, error) {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return Config{Addr: addr}, nil
}
