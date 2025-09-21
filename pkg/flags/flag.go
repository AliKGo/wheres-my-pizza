package flags

import (
	"flag"
	"log/slog"
	"os"
	"wheres-my-pizza/pkg/config"
)

var (
	defaultsPath  = "./config.yaml"
	Port          = flag.Int("port", 3000, "port to listen on")
	Mode          = flag.String("mode", "", "mode to run")
	MaxConcurrent = flag.Int("max-concurrent", 50, "maximum concurrent messages to send")
)

func ParseFlag() {
	filePath := flag.String("config", "./config.yaml", "Path to the config file")
	flag.Parse()

	if *Port < 0 || *Port > 65535 {
		slog.Error("invalid port", "port", *Port)
		os.Exit(1)
	}

	if *filePath != "" {
		*filePath = defaultsPath
	}

	config.FilePath(*filePath)
}
