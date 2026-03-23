package handlers

import (
	"github.com/airconnect/backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	Service *services.AuthService
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var input services.RegisterInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if input.Email == "" || input.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Email and password are required"})
	}

	if len(input.Password) < 8 {
		return c.Status(400).JSON(fiber.Map{"error": "Password must be at least 8 characters"})
	}

	user, tokens, err := h.Service.Register(input)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var input services.LoginInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	user, tokens, err := h.Service.Login(input)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"user":   user,
		"tokens": tokens,
	})
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	tokens, err := h.Service.RefreshToken(body.RefreshToken)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"tokens": tokens})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userId := c.Locals("userId").(string)

	var user struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		Role        string `json:"role"`
	}

	// Use raw query to avoid loading relations
	if err := services.GetUserProfile(userId, &user); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(user)
}
