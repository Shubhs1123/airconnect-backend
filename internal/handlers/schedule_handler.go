package handlers

import (
	"github.com/airconnect/backend/internal/database"
	"github.com/airconnect/backend/internal/models"
	"github.com/airconnect/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const maxDeviceSchedules = 16

type ScheduleHandler struct{}

func (h *ScheduleHandler) List(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	var schedules []models.Schedule
	if err := database.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&schedules).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(schedules)
}

func (h *ScheduleHandler) ListByDevice(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	deviceID := c.Params("deviceId")
	var schedules []models.Schedule
	if err := database.DB.Where("user_id = ? AND device_id = ?", userID, deviceID).Find(&schedules).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(schedules)
}

func (h *ScheduleHandler) Create(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	var schedule models.Schedule
	if err := c.BodyParser(&schedule); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	schedule.UserID = userID

	// Enforce device-side schedule limit
	var count int64
	database.DB.Model(&models.Schedule{}).
		Where("device_id = ? AND enabled = ?", schedule.DeviceID, true).
		Count(&count)
	if count >= maxDeviceSchedules {
		return c.Status(400).JSON(fiber.Map{
			"error": "Device supports a maximum of 16 active schedules",
		})
	}

	if err := database.DB.Create(&schedule).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Sync all schedules to device via MQTT
	go services.SyncAllSchedulesToDevice(schedule.DeviceID)

	return c.Status(201).JSON(schedule)
}

func (h *ScheduleHandler) Update(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	id := c.Params("scheduleId")
	var schedule models.Schedule
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&schedule).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Schedule not found"})
	}
	if err := c.BodyParser(&schedule); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	schedule.UserID = userID
	if err := database.DB.Save(&schedule).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Sync all schedules to device via MQTT
	go services.SyncAllSchedulesToDevice(schedule.DeviceID)

	return c.JSON(schedule)
}

func (h *ScheduleHandler) Delete(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	id := c.Params("scheduleId")
	result := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Schedule{})
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Schedule not found"})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *ScheduleHandler) DeleteByDevice(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	deviceID := c.Params("deviceId")
	database.DB.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&models.Schedule{})
	return c.JSON(fiber.Map{"ok": true})
}

