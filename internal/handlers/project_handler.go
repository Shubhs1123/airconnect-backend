package handlers

import (
	"github.com/airconnect/backend/internal/database"
	"github.com/airconnect/backend/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ProjectHandler struct{}

func (h *ProjectHandler) List(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	var projects []models.Project
	if err := database.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&projects).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(projects)
}

func (h *ProjectHandler) Create(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	var project models.Project
	if err := c.BodyParser(&project); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	project.UserID = userID
	if err := database.DB.Create(&project).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(project)
}

func (h *ProjectHandler) Get(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	id := c.Params("projectId")
	var project models.Project
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&project).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found"})
	}
	return c.JSON(project)
}

func (h *ProjectHandler) Update(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	id := c.Params("projectId")
	var project models.Project
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&project).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found"})
	}
	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	if err := database.DB.Model(&project).Updates(updates).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	database.DB.First(&project, "id = ?", id)
	return c.JSON(project)
}

func (h *ProjectHandler) Delete(c *fiber.Ctx) error {
	userID := uuid.MustParse(c.Locals("userId").(string))
	id := c.Params("projectId")
	result := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Project{})
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found"})
	}
	return c.JSON(fiber.Map{"ok": true})
}
