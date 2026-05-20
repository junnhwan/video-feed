package db

import (
	"fmt"

	"video-feed/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	switch cfg.Driver {
	case "mysql":
		return gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}

func AutoMigrate(database *gorm.DB, models ...any) error {
	if database == nil {
		return fmt.Errorf("database is nil")
	}
	return database.AutoMigrate(models...)
}
