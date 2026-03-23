package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func makeToken(userID, email, role string, expired bool) string {
	exp := time.Now().Add(time.Hour)
	if expired {
		exp = time.Now().Add(-time.Hour)
	}
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	t, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	return t
}

func newApp() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/protected", AuthRequired(testSecret), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"userId": c.Locals("userId"),
			"email":  c.Locals("email"),
			"role":   c.Locals("role"),
		})
	})
	return app
}

func TestAuthRequired_BearerHeader(t *testing.T) {
	app := newApp()
	token := makeToken("user-123", "test@example.com", "user", false)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthRequired_QueryParam(t *testing.T) {
	app := newApp()
	token := makeToken("user-456", "ws@example.com", "user", false)

	req := httptest.NewRequest("GET", "/protected?token="+token, nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for ?token= param, got %d", resp.StatusCode)
	}
}

func TestAuthRequired_Missing(t *testing.T) {
	app := newApp()
	req := httptest.NewRequest("GET", "/protected", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthRequired_Expired(t *testing.T) {
	app := newApp()
	token := makeToken("user-789", "old@example.com", "user", true)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for expired token, got %d", resp.StatusCode)
	}
}

func TestAuthRequired_WrongSecret(t *testing.T) {
	app := newApp()
	claims := JWTClaims{
		UserID: "evil",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("wrong-secret"))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for wrong secret, got %d", resp.StatusCode)
	}
}
