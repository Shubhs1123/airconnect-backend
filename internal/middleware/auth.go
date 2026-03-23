package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func AuthRequired(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Standard header auth
		tokenStr := ""
		auth := c.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		} else {
			// Fallback: query param for WebSocket upgrades (browsers can't set WS headers)
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Missing authorization"})
		}

		token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid or expired token"})
		}

		claims, ok := token.Claims.(*JWTClaims)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid token claims"})
		}

		c.Locals("userId", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}
