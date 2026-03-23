package ai

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/airconnect/backend/internal/database"
	"github.com/redis/go-redis/v9"
	openai "github.com/sashabaranov/go-openai"
)

type Service struct {
	Client *openai.Client
	Model  string
}

func NewService(apiKey, model string) *Service {
	return &Service{
		Client: openai.NewClient(apiKey),
		Model:  model,
	}
}

const firmwareSystemPrompt = `You are an ESP32 firmware configuration expert.
Given a natural language description of hardware components, generate a JSON firmware configuration.

Output strict JSON:
{
  "board": "esp32",
  "components": [{"type": "relay"|"dht22"|"pir"|"ldr"|..., "pin": number, "label": string}],
  "pinMapping": {"GPIO12": "Relay 1", ...},
  "mqttTopics": {
    "publish": ["airconnect/{device_id}/relay/1/state", ...],
    "subscribe": ["airconnect/{device_id}/relay/1/set", ...]
  },
  "conflicts": [{"pin": number, "components": ["A","B"], "resolution": "..."}],
  "warnings": ["..."],
  "firmwareFeatures": ["wifi","mqtt","ota","mdns"]
}

Rules:
- Validate GPIO constraints (pins 34-39 are input-only)
- Detect same-pin conflicts
- Suggest optimal pin assignments if conflicts found
- Use airconnect/{device_id}/... topic convention`

const wiringSystemPrompt = `You are an ESP32 wiring expert.
Given a list of components, generate wiring instructions as JSON:
{
  "steps": [{"stepNumber": 1, "instruction": "Connect...", "wireColor": "red", "safety": "optional warning"}],
  "warnings": ["..."]
}

Rules:
- Use standard wire color conventions (red=VCC, black=GND, yellow=signal)
- Include safety warnings for high-voltage components (relays)
- Recommend pull-up/pull-down resistors where needed
- Note maximum current draw per pin (40mA for ESP32)`

func (s *Service) GenerateFirmwareConfig(ctx context.Context, description string) (json.RawMessage, error) {
	// Check cache
	cacheKey := cacheKeyFor("firmware", description)
	if cached, err := getCache(ctx, cacheKey); err == nil {
		return cached, nil
	}

	resp, err := s.Client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: s.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: firmwareSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: description},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
		MaxTokens:      4096,
	})
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}

	content := resp.Choices[0].Message.Content
	result := json.RawMessage(content)

	// Cache for 24h
	setCache(ctx, cacheKey, result, 24*time.Hour)

	return result, nil
}

func (s *Service) GenerateWiringDiagram(ctx context.Context, components string) (json.RawMessage, error) {
	cacheKey := cacheKeyFor("wiring", components)
	if cached, err := getCache(ctx, cacheKey); err == nil {
		return cached, nil
	}

	resp, err := s.Client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: s.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: wiringSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: components},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
		MaxTokens:      4096,
	})
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}

	content := resp.Choices[0].Message.Content
	result := json.RawMessage(content)

	setCache(ctx, cacheKey, result, 7*24*time.Hour)

	return result, nil
}

// Redis cache helpers

func cacheKeyFor(aiType, prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("ai:cache:%s:%x", aiType, h)
}

func getCache(ctx context.Context, key string) (json.RawMessage, error) {
	if database.RDB == nil {
		return nil, fmt.Errorf("no redis")
	}
	val, err := database.RDB.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("cache miss")
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(val), nil
}

func setCache(ctx context.Context, key string, data json.RawMessage, ttl time.Duration) {
	if database.RDB == nil {
		return
	}
	database.RDB.Set(ctx, key, string(data), ttl)
}
