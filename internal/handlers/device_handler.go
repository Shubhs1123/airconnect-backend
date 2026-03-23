package handlers

import (
	"github.com/airconnect/backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type DeviceHandler struct {
	Service *services.DeviceService
}

func (h *DeviceHandler) List(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)
	devices, err := h.Service.ListDevices(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(devices)
}

func (h *DeviceHandler) Get(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)
	deviceID := c.Params("deviceId")

	device, err := h.Service.GetDevice(userID, deviceID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Device not found"})
	}
	return c.JSON(device)
}

func (h *DeviceHandler) Create(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)
	var input services.CreateDeviceInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if input.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	device, err := h.Service.CreateDevice(userID, input)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(device)
}

func (h *DeviceHandler) Update(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)
	deviceID := c.Params("deviceId")

	var input services.UpdateDeviceInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := h.Service.UpdateDevice(userID, deviceID, input); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *DeviceHandler) Delete(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)
	deviceID := c.Params("deviceId")

	if err := h.Service.DeleteDevice(userID, deviceID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
