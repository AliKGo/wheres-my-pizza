package postgresql

import (
	"fmt"
	"wheres-my-pizza/pkg/config"
)

func BuildDSN(conf config.Config) string {
	cfg := conf.Database
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.SSLMode,
	)
}
