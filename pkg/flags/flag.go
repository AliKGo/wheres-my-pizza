// pkg/flags/flag.go
package flags

import (
	"flag"
	"log/slog"
	"os"
	"wheres-my-pizza/pkg/config"
)

var (
	defaultsPath      = "./config.yaml"
	Port              = flag.Int("port", 3000, "port to listen on")
	Mode              = flag.String("mode", "", "mode to run")
	MaxConcurrent     = flag.Int("max-concurrent", 50, "maximum concurrent messages to send")
	WorkerName        = flag.String("worker-name", "", "unique name for the kitchen worker")
	OrderTypes        = flag.String("order-types", "", "comma-separated list of order types to handle (dine_in,takeout,delivery)")
	HeartbeatInterval = flag.Int("heartbeat-interval", 30, "interval in seconds between worker heartbeats")
	Prefetch          = flag.Int("prefetch", 1, "RabbitMQ prefetch count")
)

func ParseFlag() {
	filePath := flag.String("config", "./config.yaml", "Path to the config file")
	flag.Parse()

	if *Port < 0 || *Port > 65535 {
		slog.Error("invalid port", "port", *Port)
		os.Exit(1)
	}

	if *filePath != "" {
		defaultsPath = *filePath
	}

	config.FilePath(defaultsPath)
}
