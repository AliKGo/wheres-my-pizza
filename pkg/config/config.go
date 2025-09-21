package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Database struct {
		Host     string
		Port     int
		User     string
		Password string
		Database string
		SSLMode  string
	}
	RabbitMQ struct {
		Host     string
		Port     int
		User     string
		Password string
	}
}

var path = "./config.yaml"

func ParseYAML() (Config, error) {
	var cfg Config

	file, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("cannot open file: %v", err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			fmt.Printf("cannot close file: %v", err)
		}
	}()

	scanner := bufio.NewScanner(file)
	var currentSection string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasSuffix(line, ":") {
			currentSection = strings.TrimSuffix(line, ":")
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return cfg, fmt.Errorf("invalid line: %s", line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)

		if value == "" {
			return cfg, fmt.Errorf("empty value for key: %s", key)
		}

		switch currentSection {
		case "database":
			if err := setDatabaseField(&cfg, key, value); err != nil {
				return cfg, err
			}
		case "rabbitmq":
			if err := setRabbitMQField(&cfg, key, value); err != nil {
				return cfg, err
			}
		default:
			return cfg, fmt.Errorf("unknown section: %s", currentSection)
		}
	}

	if err = scanner.Err(); err != nil {
		return cfg, fmt.Errorf("error reading file: %v", err)
	}

	if err = cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func setDatabaseField(cfg *Config, key, value string) error {
	switch key {
	case "host":
		cfg.Database.Host = value
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("invalid port for database: %s", value)
		}
		cfg.Database.Port = port
	case "user":
		cfg.Database.User = value
	case "password":
		cfg.Database.Password = value
	case "database":
		cfg.Database.Database = value
	case "sslmode":
		cfg.Database.SSLMode = value
	default:
		return fmt.Errorf("unknown database key: %s", key)
	}
	return nil
}

func setRabbitMQField(cfg *Config, key, value string) error {
	switch key {
	case "host":
		cfg.RabbitMQ.Host = value
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("invalid port for rabbitmq: %s", value)
		}
		cfg.RabbitMQ.Port = port
	case "user":
		cfg.RabbitMQ.User = value
	case "password":
		cfg.RabbitMQ.Password = value
	default:
		return fmt.Errorf("unknown rabbitmq key: %s", key)
	}
	return nil
}

func PrintYAMLHelp() {
	fmt.Printf(`
YAML Config File Help

Your configuration file should be written in YAML format. It supports two main sections: database and rabbitmq. Each section contains key-value pairs.

Example config.yaml:

database:
  host: "localhost"        # Database host
  port: 5432               # Database port (integer 1-65535)
  user: "postgres"         # Database username
  password: "secret"       # Database password
  database: "mydb"         # Database name
  sslmode: "disable"       # SSL mode: disable/enable

rabbitmq:
  host: "localhost"        # RabbitMQ host
  port: 5672               # RabbitMQ port (integer 1-65535)
  user: "guest"            # RabbitMQ username
  password: "guest"        # RabbitMQ password

Rules:
1. Sections: Only "database" and "rabbitmq" are supported.
2. Key-Value Pairs: Use key: value format. Indentation must be consistent (spaces only, no tabs).
3. Strings: You can use quotes (" or ') or leave them unquoted.
4. Integers: Port values must be numeric and between 1–65535.
5. Comments: Start a comment with #. Everything after # on the line is ignored.
6. Empty Values: Not allowed. Every key must have a value.
`)
}

func FilePath(filePath string) {
	path = filePath
}

func (c *Config) validate() error {
	// Database
	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.Port == 0 {
		return fmt.Errorf("database.port is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if c.Database.Password == "" {
		return fmt.Errorf("database.password is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("database.database is required")
	}
	if c.Database.SSLMode == "" {
		return fmt.Errorf("database.sslmode is required")
	}

	// RabbitMQ
	if c.RabbitMQ.Host == "" {
		return fmt.Errorf("rabbitmq.host is required")
	}
	if c.RabbitMQ.Port == 0 {
		return fmt.Errorf("rabbitmq.port is required")
	}
	if c.RabbitMQ.User == "" {
		return fmt.Errorf("rabbitmq.user is required")
	}
	if c.RabbitMQ.Password == "" {
		return fmt.Errorf("rabbitmq.password is required")
	}

	return nil
}
