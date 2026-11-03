// @title Task Management Multi-Level API
// @version 1.1
// @description Đặc tả API ứng dụng quản lý công việc nhiều cấp (Golang Fiber + PostgreSQL)
// @termsOfService http://swagger.io/terms/

// @contact.name Đội ngũ phát triển
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Nhập token theo cú pháp: Bearer {your_jwt_token}

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"

	"taskmgmt/internal/config"
	"taskmgmt/internal/database"
	"taskmgmt/internal/platform/response"
	"taskmgmt/internal/router"
)

func main() {
	cfg := config.Load()

	log.Printf("Starting Task Management API (env=%s, port=%s)...", cfg.AppEnv, cfg.Port)

	// Connect to Database
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}

	// Auto-migrate schema in development if enabled
	if cfg.AutoMigrate {
		if err := database.AutoMigrate(db); err != nil {
			log.Fatalf("Database migration failed: %v", err)
		}
		log.Println("Database schema checked and verified successfully")
	}

	app := fiber.New(fiber.Config{
		AppName:      "Task Management Multi-Level API v1.1",
		ServerHeader: "Fiber",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			codeStr := "INTERNAL_ERROR"
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				if code == fiber.StatusNotFound {
					codeStr = "NOT_FOUND"
				} else if code == fiber.StatusBadRequest {
					codeStr = "BAD_REQUEST"
				}
			}
			return response.Error(c, code, codeStr, err.Error(), nil)
		},
	})

	router.Setup(app, db, cfg)

	// Graceful shutdown
	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Printf("Server shut down: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")
	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exiting")
}

