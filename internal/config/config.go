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
	if err := yaml.Unmarshal(payload, &cfg); err != nil {
		return Config{}, err
	}
	normalize(&cfg)
	return cfg, nil
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
