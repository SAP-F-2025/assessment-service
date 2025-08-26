package pkg

import (
	"fmt"

	"github.com/SAP-F-2025/assessment-service/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDatabase(cfg *config.Config) (*gorm.DB, error) {
	var logLevel logger.LogLevel
	if cfg.Environment == "production" {
		logLevel = logger.Info
	} else {
		logLevel = logger.Error
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}
