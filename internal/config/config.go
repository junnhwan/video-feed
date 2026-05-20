package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Redis         RedisConfig         `yaml:"redis"`
	RabbitMQ      RabbitMQConfig      `yaml:"rabbitmq"`
	Observability ObservabilityConfig `yaml:"observability"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type DatabaseConfig struct {
	Driver   string `yaml:"driver"`
	DSN      string `yaml:"dsn"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type RabbitMQConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ObservabilityConfig struct {
	Pprof PprofConfig `yaml:"pprof"`
}

type PprofConfig struct {
	Enabled    bool   `yaml:"enabled"`
	APIAddr    string `yaml:"api_addr"`
	WorkerAddr string `yaml:"worker_addr"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Database: DatabaseConfig{
			Driver: "mysql",
			DSN:    "root:123456@tcp(127.0.0.1:3306)/video_feed?charset=utf8mb4&parseTime=True&loc=Local",
			Host:   "127.0.0.1",
			Port:   3306,
			User:   "root",
			DBName: "video_feed",
		},
		Redis: RedisConfig{
			Host:     "127.0.0.1",
			Port:     6379,
			Password: "",
			DB:       0,
		},
		RabbitMQ: RabbitMQConfig{
			Host:     "127.0.0.1",
			Port:     5672,
			Username: "admin",
			Password: "password123",
		},
		Observability: ObservabilityConfig{
			Pprof: PprofConfig{
				Enabled:    false,
				APIAddr:    "127.0.0.1:6060",
				WorkerAddr: "127.0.0.1:6061",
			},
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var fileCfg Config
	if err := yaml.Unmarshal(payload, &fileCfg); err != nil {
		return Config{}, err
	}
	merge(&cfg, fileCfg)
	normalize(&cfg)
	return cfg, nil
}

func merge(cfg *Config, fileCfg Config) {
	if fileCfg.Server.Port != 0 {
		cfg.Server.Port = fileCfg.Server.Port
	}
	if fileCfg.Database.Driver != "" {
		cfg.Database.Driver = fileCfg.Database.Driver
	}
	if fileCfg.Database.DSN != "" {
		cfg.Database.DSN = fileCfg.Database.DSN
	} else if fileCfg.Database.Host != "" || fileCfg.Database.Port != 0 || fileCfg.Database.User != "" || fileCfg.Database.Password != "" || fileCfg.Database.DBName != "" {
		cfg.Database.DSN = ""
	}
	if fileCfg.Database.Host != "" {
		cfg.Database.Host = fileCfg.Database.Host
	}
	if fileCfg.Database.Port != 0 {
		cfg.Database.Port = fileCfg.Database.Port
	}
	if fileCfg.Database.User != "" {
		cfg.Database.User = fileCfg.Database.User
	}
	if fileCfg.Database.Password != "" {
		cfg.Database.Password = fileCfg.Database.Password
	}
	if fileCfg.Database.DBName != "" {
		cfg.Database.DBName = fileCfg.Database.DBName
	}
	if fileCfg.Redis.Host != "" {
		cfg.Redis.Host = fileCfg.Redis.Host
	}
	if fileCfg.Redis.Port != 0 {
		cfg.Redis.Port = fileCfg.Redis.Port
	}
	if fileCfg.Redis.Password != "" {
		cfg.Redis.Password = fileCfg.Redis.Password
	}
	if fileCfg.Redis.DB != 0 {
		cfg.Redis.DB = fileCfg.Redis.DB
	}
	if fileCfg.RabbitMQ.Host != "" {
		cfg.RabbitMQ.Host = fileCfg.RabbitMQ.Host
	}
	if fileCfg.RabbitMQ.Port != 0 {
		cfg.RabbitMQ.Port = fileCfg.RabbitMQ.Port
	}
	if fileCfg.RabbitMQ.Username != "" {
		cfg.RabbitMQ.Username = fileCfg.RabbitMQ.Username
	}
	if fileCfg.RabbitMQ.Password != "" {
		cfg.RabbitMQ.Password = fileCfg.RabbitMQ.Password
	}
	cfg.Observability.Pprof.Enabled = fileCfg.Observability.Pprof.Enabled
	if fileCfg.Observability.Pprof.APIAddr != "" {
		cfg.Observability.Pprof.APIAddr = fileCfg.Observability.Pprof.APIAddr
	}
	if fileCfg.Observability.Pprof.WorkerAddr != "" {
		cfg.Observability.Pprof.WorkerAddr = fileCfg.Observability.Pprof.WorkerAddr
	}
}

func normalize(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "mysql"
	}
	if cfg.Database.DSN == "" && cfg.Database.Host != "" {
		cfg.Database.DSN = fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.DBName,
		)
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = 6379
	}
	if cfg.RabbitMQ.Port == 0 {
		cfg.RabbitMQ.Port = 5672
	}
}
