package config

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
}

type ServerConfig struct {
	Port int
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type RabbitMQConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Database: DatabaseConfig{
			Driver: "mysql",
			DSN:    "root:123456@tcp(127.0.0.1:3306)/video_feed?charset=utf8mb4&parseTime=True&loc=Local",
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
	}
}
