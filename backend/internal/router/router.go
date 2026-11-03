package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
	"gorm.io/gorm"

	_ "taskmgmt/docs"
	"taskmgmt/internal/config"
	"taskmgmt/internal/middleware"
	authModule "taskmgmt/internal/modules/auth"
	deptModule "taskmgmt/internal/modules/departments"
	menuModule "taskmgmt/internal/modules/menus"
	notifModule "taskmgmt/internal/modules/notifications"
	rbacModule "taskmgmt/internal/modules/rbac"
	reportsModule "taskmgmt/internal/modules/reports"
	taskModule "taskmgmt/internal/modules/tasks"
	userModule "taskmgmt/internal/modules/users"
	"taskmgmt/internal/platform/response"
)

func Setup(app *fiber.App, db *gorm.DB, cfg *config.Config) {
	// Global Middlewares
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path} (${locals:requestid})\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			return true
		},
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, Idempotency-Key",
		AllowMethods:     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		AllowCredentials: true,
	}))
	app.Use(middleware.RequestID())

	// Health Check
	app.Get("/health", func(c *fiber.Ctx) error {
		return response.JSON(c, fiber.StatusOK, fiber.Map{
			"status":  "ok",
			"version": "v1.1",
		})
	})

	// Swagger & OpenAPI 3.0 Documentation
	app.Get("/swagger/openapi.json", func(c *fiber.Ctx) error {
		return c.SendFile("./docs/openapi.json")
	})
	app.Get("/openapi.json", func(c *fiber.Ctx) error {
		return c.SendFile("./docs/openapi.json")
	})
	app.Get("/swagger/*", swagger.HandlerDefault)

	api := app.Group("/api/v1")

	// Auth Middleware
	authCheck := middleware.AuthRequired(cfg.JWTSecret)

	// Auth Module
	authSvc := authModule.NewService(db, cfg)
	authHdl := authModule.NewHandler(authSvc)
	authModule.RegisterRoutes(api, authHdl, authCheck)
	authModule.RegisterRoutes(app, authHdl, authCheck)

	// Notifications Module
	notifRepo := notifModule.NewRepository(db)
	notifSvc := notifModule.NewService(notifRepo)
	notifHdl := notifModule.NewHandler(notifSvc)
	notifModule.RegisterRoutes(api, notifHdl, authCheck)
	notifModule.RegisterRoutes(app, notifHdl, authCheck)

	// Tasks Module
	taskRepo := taskModule.NewRepository(db)
	taskSvc := taskModule.NewService(taskRepo, notifSvc)
	taskHdl := taskModule.NewHandler(taskSvc)
	taskModule.RegisterRoutes(api, taskHdl, authCheck)
	taskModule.RegisterRoutes(app, taskHdl, authCheck)

	// RBAC Module
	rbacRepo := rbacModule.NewRepository(db)
	rbacSvc := rbacModule.NewService(rbacRepo)
	rbacHdl := rbacModule.NewHandler(rbacSvc)
	rbacModule.RegisterRoutes(api, rbacHdl, authCheck)
	rbacModule.RegisterRoutes(app, rbacHdl, authCheck)

	// Menus Module
	menuRepo := menuModule.NewRepository(db)
	menuSvc := menuModule.NewService(menuRepo)
	menuHdl := menuModule.NewHandler(menuSvc, rbacSvc)
	menuModule.RegisterRoutes(api, menuHdl, authCheck)
	menuModule.RegisterRoutes(app, menuHdl, authCheck)

	// Users Module
	userRepo := userModule.NewRepository(db)
	userSvc := userModule.NewService(userRepo)
	userHdl := userModule.NewHandler(userSvc)
	userModule.RegisterRoutes(api, userHdl, authCheck)
	userModule.RegisterRoutes(app, userHdl, authCheck)

	// Departments & Organization Module
	deptRepo := deptModule.NewRepository(db)
	deptSvc := deptModule.NewService(deptRepo)
	deptHdl := deptModule.NewHandler(deptSvc)
	deptModule.RegisterRoutes(api, deptHdl, authCheck)
	deptModule.RegisterRoutes(app, deptHdl, authCheck)

	// Dashboard & Reports Module
	reportsRepo := reportsModule.NewRepository(db)
	reportsSvc := reportsModule.NewService(reportsRepo)
	reportsHdl := reportsModule.NewHandler(reportsSvc)
	reportsModule.RegisterRoutes(api, reportsHdl, authCheck)
	reportsModule.RegisterRoutes(app, reportsHdl, authCheck)
}

