package database

import (
	"context"
	"log"

	"github.com/airconnect/backend/internal/models"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
)

func InitPostgres(dsn string) {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate
	err = DB.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.TelemetryRecord{},
		&models.FirmwareTemplate{},
		&models.FirmwareBuild{},
		&models.AutomationRule{},
		&models.AutomationLog{},
		&models.AIInteraction{},
		&models.Project{},
		&models.Schedule{},
	)
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database connected and migrated")
}

func InitRedis(url string) {
	if url == "" {
		log.Println("Redis URL not set — skipping Redis (AI caching disabled)")
		return
	}

	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("Failed to parse Redis URL: %v — skipping Redis", err)
		return
	}

	RDB = redis.NewClient(opt)
	if err := RDB.Ping(context.Background()).Err(); err != nil {
		log.Printf("Failed to connect to Redis: %v — skipping Redis", err)
		RDB = nil
		return
	}

	log.Println("Redis connected")
}
