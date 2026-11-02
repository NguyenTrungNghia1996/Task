package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"taskmgmt/internal/config"
	"taskmgmt/internal/model"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Ho_Chi_Minh",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)

	gormLogLevel := logger.Warn
	if cfg.DBLogSQL {
		gormLogLevel = logger.Info
	}

	// Đặt ngưỡng cảnh báo Slow SQL:
	// Khi chạy local kết nối tới VPS Database từ xa, độ trễ mạng vật lý Internet (RTT) mất ~120-220ms.
	// Đặt ngưỡng 500ms cho kết nối remote để tránh cảnh báo nhầm do độ trễ đường truyền mạng.
	slowThreshold := 200 * time.Millisecond
	if cfg.DBHost != "localhost" && cfg.DBHost != "127.0.0.1" {
		slowThreshold = 500 * time.Millisecond
	}

	customLogger := logger.New(
		log.New(log.Writer(), "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             slowThreshold,
			LogLevel:                  gormLogLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 customLogger,
		SkipDefaultTransaction: true, // Bỏ transaction mặc định cho single create/update giúp giảm số lượt gửi nhận mạng (RTT)
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB handle: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("Database connection established successfully")
	return db, nil
}

// AutoMigrate executes schema migrations for development and testing.
func AutoMigrate(db *gorm.DB) error {
	// Dùng session với log mode Silent/Warn để tránh in hàng chục câu query pg_catalog mỗi lần khởi động
	silentDB := db.Session(&gorm.Session{
		Logger: logger.Default.LogMode(logger.Warn),
	})

	return silentDB.AutoMigrate(
		&model.Organization{},
		&model.Department{},
		&model.User{},
		&model.Role{},
		&model.UserRole{},
		&model.DepartmentMember{},
		&model.Task{},
		&model.TaskAssignment{},
		&model.TaskUpdate{},
		&model.TaskReview{},
		&model.TaskComment{},
		&model.TaskEvent{},
		&model.TaskAttachment{},
		&model.Notification{},
		&model.RefreshToken{},
		&model.IdempotencyKey{},
		&model.Permission{},
		&model.RolePermission{},
		&model.Menu{},
	)
}

