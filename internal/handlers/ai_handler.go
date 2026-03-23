package handlers

import (
	"github.com/airconnect/backend/internal/ai"
	"github.com/gofiber/fiber/v2"
)

type AIHandler struct {
	Service *ai.Service
}

func (h *AIHandler) GenerateFirmwareConfig(c *fiber.Ctx) error {
	var body struct {
		Description string `json:"description"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if body.Description == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Description is required"})
	}

	result, err := h.Service.GenerateFirmwareConfig(c.Context(), body.Description)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Send(result)
}

func (h *AIHandler) GenerateWiringDiagram(c *fiber.Ctx) error {
	var body struct {
		Components string `json:"components"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if body.Components == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Components list is required"})
	}

	result, err := h.Service.GenerateWiringDiagram(c.Context(), body.Components)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Send(result)
}
