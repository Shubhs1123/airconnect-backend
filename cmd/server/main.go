package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/airconnect/backend/internal/ai"
	"github.com/airconnect/backend/internal/config"
	"github.com/airconnect/backend/internal/database"
	"github.com/airconnect/backend/internal/handlers"
	"github.com/airconnect/backend/internal/middleware"
	"github.com/airconnect/backend/internal/models"
	"github.com/airconnect/backend/internal/mqtt"
	"github.com/airconnect/backend/internal/services"
	appws "github.com/airconnect/backend/internal/ws"
	"github.com/gofiber/fiber/v2"
	fiberws "github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg := config.Load()

	// Initialize databases
	database.InitPostgres(cfg.DatabaseURL)
	database.InitRedis(cfg.RedisURL)

	// Initialize MQTT
	mqtt.Connect(mqtt.Config{
		BrokerURL: cfg.MQTTBrokerURL,
		Username:  cfg.MQTTAdminUser,
		Password:  cfg.MQTTAdminPass,
	})

	// Wire up MQTT device lifecycle callbacks
	mqtt.OnDeviceOnline = services.SyncAllSchedulesToDevice
	mqtt.OnDeviceOffline = services.MarkSchedulesUnsynced

	// Initialize services
	authService := &services.AuthService{
		JWTSecret:        cfg.JWTSecret,
		JWTRefreshSecret: cfg.JWTRefreshSecret,
	}
	deviceService := &services.DeviceService{}
	aiService := ai.NewService(cfg.OpenAIKey, cfg.OpenAIModel)

	// Initialize handlers
	authHandler := &handlers.AuthHandler{Service: authService}
	deviceHandler := &handlers.DeviceHandler{Service: deviceService}
	aiHandler := &handlers.AIHandler{Service: aiService}
	projectHandler := &handlers.ProjectHandler{}
	scheduleHandler := &handlers.ScheduleHandler{}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:   "AIRConnect Backend",
		BodyLimit: 50 * 1024 * 1024, // 50MB for firmware uploads
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "airconnect-backend",
		})
	})

	// API routes
	api := app.Group("/api/v1")

	// Auth (public)
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.Refresh)

	// Protected routes
	protected := api.Group("", middleware.AuthRequired(cfg.JWTSecret))

	// Auth - protected
	protected.Get("/auth/me", authHandler.Me)

	// WebSocket endpoint — authenticated via ?token= query param
	app.Get("/ws", middleware.AuthRequired(cfg.JWTSecret), fiberws.New(func(c *fiberws.Conn) {
		userID := c.Locals("userId").(string)
		appws.Default.Serve(c, userID)
	}))

	// Devices
	devices := protected.Group("/devices")
	devices.Get("/", deviceHandler.List)
	devices.Post("/", deviceHandler.Create)
	devices.Get("/:deviceId", deviceHandler.Get)
	devices.Put("/:deviceId", deviceHandler.Update)
	devices.Delete("/:deviceId", deviceHandler.Delete)

	// Relay command via MQTT
	devices.Post("/:deviceId/relay", func(c *fiber.Ctx) error {
		userID   := c.Locals("userId").(string)
		deviceID := c.Params("deviceId")
		var body struct {
			Relay int    `json:"relay"`
			State string `json:"state"` // "ON", "OFF", "TOGGLE"
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
		}
		var device models.Device
		if err := database.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Device not found"})
		}

		mqttName := device.MQTTUsername
		if mqttName == "" { mqttName = device.Name }
		if mqttName == "" {
			return c.Status(503).JSON(fiber.Map{"error": "Device has no MQTT name configured"})
		}

		// MQTT relay topics use 1-based indexing
		relayNum := body.Relay + 1
		if err := mqtt.PublishRelayCommand(mqttName, relayNum, body.State); err != nil {
			return c.Status(503).JSON(fiber.Map{"error": "MQTT publish failed: " + err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true, "relay": body.Relay, "via": "mqtt"})
	})

	// GPIO control via MQTT
	devices.Post("/:deviceId/gpio", func(c *fiber.Ctx) error {
		userID   := c.Locals("userId").(string)
		deviceID := c.Params("deviceId")
		var body struct {
			Pin   int  `json:"pin"`
			State bool `json:"state"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
		}
		var device models.Device
		if err := database.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Device not found"})
		}

		mqttName := device.MQTTUsername
		if mqttName == "" { mqttName = device.Name }
		if mqttName == "" {
			return c.Status(503).JSON(fiber.Map{"error": "Device has no MQTT name configured"})
		}

		stateStr := "OFF"
		if body.State { stateStr = "ON" }
		gpioCmd := fmt.Sprintf(`{"cmd":"set_gpio","pin":%d,"state":"%s"}`, body.Pin, stateStr)
		if err := mqtt.PublishScheduleCommand(mqttName, gpioCmd); err != nil {
			return c.Status(503).JSON(fiber.Map{"error": "MQTT publish failed: " + err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true, "pin": body.Pin, "state": body.State, "via": "mqtt"})
	})

	// Push config to device via MQTT
	devices.Post("/:deviceId/config", func(c *fiber.Ctx) error {
		userID   := c.Locals("userId").(string)
		deviceID := c.Params("deviceId")
		var device models.Device
		if err := database.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Device not found"})
		}

		mqttName := device.MQTTUsername
		if mqttName == "" { mqttName = device.Name }
		if mqttName == "" {
			return c.Status(503).JSON(fiber.Map{"error": "Device has no MQTT name configured"})
		}

		configCmd := fmt.Sprintf(`{"cmd":"set_config","config":%s}`, string(c.Body()))
		if err := mqtt.PublishScheduleCommand(mqttName, configCmd); err != nil {
			return c.Status(503).JSON(fiber.Map{"error": "MQTT publish failed: " + err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true, "via": "mqtt"})
	})

	// Legacy command endpoint
	devices.Post("/:deviceId/command", func(c *fiber.Ctx) error {
		deviceID := c.Params("deviceId")
		var body struct {
			Command string `json:"command"`
			Payload string `json:"payload"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
		}
		if err := mqtt.SendCommand(deviceID, body.Command, body.Payload); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	})

	// Projects
	projects := protected.Group("/projects")
	projects.Get("/", projectHandler.List)
	projects.Post("/", projectHandler.Create)
	projects.Get("/:projectId", projectHandler.Get)
	projects.Put("/:projectId", projectHandler.Update)
	projects.Delete("/:projectId", projectHandler.Delete)

	// Schedules
	schedules := protected.Group("/schedules")
	schedules.Get("/", scheduleHandler.List)
	schedules.Post("/", scheduleHandler.Create)
	schedules.Put("/:scheduleId", scheduleHandler.Update)
	schedules.Delete("/:scheduleId", scheduleHandler.Delete)
	devices.Get("/:deviceId/schedules", scheduleHandler.ListByDevice)
	devices.Delete("/:deviceId/schedules", scheduleHandler.DeleteByDevice)

	// AI
	aiGroup := protected.Group("/ai")
	aiGroup.Post("/firmware-config", aiHandler.GenerateFirmwareConfig)
	aiGroup.Post("/wiring-diagram", aiHandler.GenerateWiringDiagram)

	// Automations
	automations := protected.Group("/automations")
	automations.Get("/", func(c *fiber.Ctx) error {
		// TODO: list automations
		return c.JSON([]interface{}{})
	})
	automations.Post("/", func(c *fiber.Ctx) error {
		// TODO: create automation
		return c.Status(201).JSON(fiber.Map{"ok": true})
	})

	// ── Remote OTA: App uploads firmware → Backend stores → ESP32 pulls via HTTP ──

	// In-memory map of download tokens → file paths (auto-expire after 10 min)
	type otaEntry struct {
		filePath string
		expires  time.Time
	}
	otaTokens := &sync.Map{}

	// Cleanup expired tokens every 2 minutes
	go func() {
		for {
			time.Sleep(2 * time.Minute)
			otaTokens.Range(func(key, value interface{}) bool {
				if e, ok := value.(*otaEntry); ok && time.Now().After(e.expires) {
					os.Remove(e.filePath)
					otaTokens.Delete(key)
				}
				return true
			})
		}
	}()

	// POST /api/v1/devices/:deviceId/ota — upload firmware, tell device to pull it
	devices.Post("/:deviceId/ota", func(c *fiber.Ctx) error {
		userID := c.Locals("userId").(string)
		deviceID := c.Params("deviceId")

		var device models.Device
		if err := database.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Device not found"})
		}

		mqttName := device.MQTTUsername
		if mqttName == "" {
			mqttName = device.Name
		}
		if mqttName == "" {
			return c.Status(503).JSON(fiber.Map{"error": "Device has no MQTT name"})
		}
		if !device.IsOnline {
			return c.Status(503).JSON(fiber.Map{"error": "Device is offline"})
		}

		// Read firmware file from multipart form
		file, err := c.FormFile("firmware")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "No firmware file provided"})
		}

		// Save to storage dir with random token filename
		tokenBytes := make([]byte, 16)
		rand.Read(tokenBytes)
		dlToken := hex.EncodeToString(tokenBytes)

		otaDir := filepath.Join(cfg.StoragePath, "ota")
		os.MkdirAll(otaDir, 0755)
		filePath := filepath.Join(otaDir, dlToken+".bin")

		if err := c.SaveFile(file, filePath); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save firmware file"})
		}

		// Store download token (expires in 10 min)
		otaTokens.Store(dlToken, &otaEntry{
			filePath: filePath,
			expires:  time.Now().Add(10 * time.Minute),
		})

		// Build the public download URL for the ESP32 to fetch
		host := c.Get("X-Forwarded-Host", c.Hostname())
		scheme := c.Get("X-Forwarded-Proto", c.Protocol())
		downloadURL := fmt.Sprintf("%s://%s/api/v1/firmware/download/%s", scheme, host, dlToken)

		// Send MQTT command to device
		if err := mqtt.PublishOTACommand(mqttName, downloadURL); err != nil {
			os.Remove(filePath)
			otaTokens.Delete(dlToken)
			return c.Status(503).JSON(fiber.Map{"error": "MQTT publish failed: " + err.Error()})
		}

		log.Printf("[OTA] Firmware uploaded for device %s (%s), download token: %s", device.Name, deviceID, dlToken)
		return c.JSON(fiber.Map{
			"ok":          true,
			"message":     "Firmware uploaded. Device will download and flash it.",
			"downloadUrl": downloadURL,
			"via":         "mqtt",
		})
	})

	// GET /api/v1/firmware/download/:token — ESP32 fetches firmware binary (no auth)
	api.Get("/firmware/download/:token", func(c *fiber.Ctx) error {
		dlToken := c.Params("token")
		entry, ok := otaTokens.Load(dlToken)
		if !ok {
			return c.Status(404).JSON(fiber.Map{"error": "Invalid or expired download token"})
		}
		e := entry.(*otaEntry)
		if time.Now().After(e.expires) {
			os.Remove(e.filePath)
			otaTokens.Delete(dlToken)
			return c.Status(410).JSON(fiber.Map{"error": "Download link expired"})
		}
		c.Set("Content-Type", "application/octet-stream")
		return c.SendFile(e.filePath)
	})

	// Firmware
	firmware := protected.Group("/firmware")
	firmware.Get("/templates", func(c *fiber.Ctx) error {
		// TODO: list templates
		return c.JSON([]interface{}{})
	})
	firmware.Post("/build", func(c *fiber.Ctx) error {
		// TODO: queue build
		return c.Status(202).JSON(fiber.Map{"status": "queued"})
	})

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("AIRConnect backend starting on %s", addr)
	log.Fatal(app.Listen(addr))
}
