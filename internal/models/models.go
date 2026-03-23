package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Base model with UUID primary key
type Base struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// User
type User struct {
	Base
	Email        string `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string `gorm:"not null" json:"-"`
	DisplayName  string `json:"displayName"`
	Role         string `gorm:"default:user" json:"role"`
	FCMToken     string `json:"fcmToken,omitempty"`

	Devices    []Device         `gorm:"foreignKey:UserID" json:"-"`
	Automations []AutomationRule `gorm:"foreignKey:UserID" json:"-"`
}

// Device
type Device struct {
	Base
	UserID          uuid.UUID      `gorm:"type:uuid;not null" json:"userId"`
	Name            string         `gorm:"not null" json:"name"`
	ChipType        string         `json:"chipType"`
	MACAddress      string         `gorm:"uniqueIndex" json:"macAddress"`
	FirmwareVersion string         `json:"firmwareVersion"`
	HardwareConfig  datatypes.JSON `json:"hardwareConfig"`
	MQTTUsername    string         `json:"mqttUsername,omitempty"`
	MQTTPassword    string         `json:"-"`
	APIToken        string         `json:"apiToken"`
	LastSeen        *time.Time     `json:"lastSeen"`
	IsOnline        bool           `gorm:"default:false" json:"isOnline"`
	IPAddress       string         `json:"ipAddress"`
	Port            int            `gorm:"default:80" json:"port"`
	Status          string         `gorm:"default:registered" json:"status"`
}

// TelemetryRecord
type TelemetryRecord struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	DeviceID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"deviceId"`
	Data      datatypes.JSON `json:"data"`
	Timestamp time.Time      `gorm:"index" json:"timestamp"`
}

func (t *TelemetryRecord) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// FirmwareTemplate
type FirmwareTemplate struct {
	Base
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	ChipType     string         `json:"chipType"`
	Version      string         `json:"version"`
	ConfigSchema datatypes.JSON `json:"configSchema"`
	SourceDir    string         `json:"sourceDir"`
	IsActive     bool           `gorm:"default:true" json:"isActive"`
}

// FirmwareBuild
type FirmwareBuild struct {
	Base
	UserID      uuid.UUID      `gorm:"type:uuid;not null" json:"userId"`
	DeviceID    *uuid.UUID     `gorm:"type:uuid" json:"deviceId"`
	TemplateID  uuid.UUID      `gorm:"type:uuid;not null" json:"templateId"`
	Config      datatypes.JSON `json:"config"`
	Status      string         `gorm:"default:queued" json:"status"` // queued, building, success, failed
	BinPath     string         `json:"binPath,omitempty"`
	BuildLog    string         `gorm:"type:text" json:"buildLog,omitempty"`
	Version     string         `json:"version"`
	FileSize    int64          `json:"fileSize"`
	Checksum    string         `json:"checksum"`
	StartedAt   *time.Time     `json:"startedAt"`
	CompletedAt *time.Time     `json:"completedAt"`
}

// AutomationRule
type AutomationRule struct {
	Base
	UserID      uuid.UUID      `gorm:"type:uuid;not null" json:"userId"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `json:"description"`
	Trigger     datatypes.JSON `json:"trigger"`
	Actions     datatypes.JSON `json:"actions"`
	Schedule    datatypes.JSON `json:"schedule,omitempty"`
	IsEnabled   bool           `gorm:"default:true" json:"isEnabled"`
	Priority    int            `gorm:"default:0" json:"priority"`
	AIGenerated bool           `gorm:"default:false" json:"aiGenerated"`
}

// AutomationLog
type AutomationLog struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	RuleID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"ruleId"`
	Status    string         `json:"status"`
	Input     datatypes.JSON `json:"input,omitempty"`
	Output    datatypes.JSON `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
	Timestamp time.Time      `gorm:"index" json:"timestamp"`
}

func (a *AutomationLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Project — groups device + outputs + inputs + widgets
type Project struct {
	Base
	UserID        uuid.UUID      `gorm:"type:uuid;not null" json:"userId"`
	Name          string         `gorm:"not null" json:"name"`
	DeviceID      uuid.UUID      `gorm:"type:uuid;not null" json:"deviceId"`
	DeviceName    string         `json:"deviceName"`
	DeviceIP      string         `json:"deviceIp"`
	Outputs       datatypes.JSON `json:"outputs"`       // []string
	Inputs        datatypes.JSON `json:"inputs"`        // []string
	Pins          datatypes.JSON `json:"pins"`          // map[string]int
	WidgetConfigs datatypes.JSON `json:"widgetConfigs"` // map[string]WidgetConfig
	CondValue     string         `json:"condValue,omitempty"`
	ScheduleTime  string         `json:"scheduleTime,omitempty"`
	AutomationID  string         `json:"automationId,omitempty"`
}

// Schedule — time-based trigger stored in backend
type Schedule struct {
	Base
	UserID   uuid.UUID `gorm:"type:uuid;not null" json:"userId"`
	DeviceID uuid.UUID `gorm:"type:uuid;not null" json:"deviceId"`
	Hour     int       `json:"hour"`
	Minute   int       `json:"minute"`
	Days     int       `json:"days"` // bitmask 0x7F = every day
	Relay    int       `json:"relay"`
	GPIO     int       `gorm:"default:-1" json:"gpio"` // -1 = use relay, >=0 = direct GPIO pin
	State    bool      `json:"state"`
	Enabled  bool      `gorm:"default:true" json:"enabled"`
	Synced   bool      `gorm:"default:false" json:"synced"` // pushed to ESP32?
}

// AIInteraction
type AIInteraction struct {
	Base
	UserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	Type       string    `json:"type"` // firmware_config, wiring_diagram, troubleshoot, chat
	Prompt     string    `gorm:"type:text" json:"prompt"`
	Response   string    `gorm:"type:text" json:"response"`
	Model      string    `json:"model"`
	TokensUsed int       `json:"tokensUsed"`
	Cached     bool      `gorm:"default:false" json:"cached"`
}
