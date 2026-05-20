package config

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

type ServerConfig struct {
	Port int
}

type DatabaseConfig struct {
	Driver string
	DSN    string
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
	}
}
