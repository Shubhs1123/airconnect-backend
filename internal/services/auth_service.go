package services

import (
	"errors"
	"time"

	"github.com/airconnect/backend/internal/database"
	"github.com/airconnect/backend/internal/middleware"
	"github.com/airconnect/backend/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	JWTSecret        string
	JWTRefreshSecret string
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type RegisterInput struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *AuthService) Register(input RegisterInput) (*models.User, *TokenPair, error) {
	// Check existing
	var existing models.User
	if err := database.DB.Where("email = ?", input.Email).First(&existing).Error; err == nil {
		return nil, nil, errors.New("email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}

	user := models.User{
		Base:         models.Base{ID: uuid.New()},
		Email:        input.Email,
		PasswordHash: string(hash),
		DisplayName:  input.DisplayName,
		Role:         "user",
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return nil, nil, err
	}

	tokens, err := s.generateTokens(&user)
	if err != nil {
		return nil, nil, err
	}

	return &user, tokens, nil
}

func (s *AuthService) Login(input LoginInput) (*models.User, *TokenPair, error) {
	var user models.User
	if err := database.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		return nil, nil, errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, nil, errors.New("invalid email or password")
	}

	tokens, err := s.generateTokens(&user)
	if err != nil {
		return nil, nil, err
	}

	return &user, tokens, nil
}

func (s *AuthService) RefreshToken(refreshToken string) (*TokenPair, error) {
	token, err := jwt.ParseWithClaims(refreshToken, &middleware.JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.JWTRefreshSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid refresh token")
	}

	claims, ok := token.Claims.(*middleware.JWTClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
		return nil, errors.New("user not found")
	}

	return s.generateTokens(&user)
}

func (s *AuthService) generateTokens(user *models.User) (*TokenPair, error) {
	now := time.Now()
	accessExpiry := now.Add(15 * time.Minute)

	accessClaims := middleware.JWTClaims{
		UserID: user.ID.String(),
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.JWTSecret))
	if err != nil {
		return nil, err
	}

	refreshClaims := middleware.JWTClaims{
		UserID: user.ID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.JWTRefreshSecret))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(15 * 60),
	}, nil
}
