package services

import (
	"github.com/airconnect/backend/internal/database"
	"github.com/airconnect/backend/internal/models"
	"github.com/google/uuid"
)

func GetUserProfile(userId string, dest interface{}) error {
	return database.DB.Model(&models.User{}).
		Select("id, email, display_name, role").
		Where("id = ?", userId).
		Scan(dest).Error
}

type DeviceService struct{}

func (s *DeviceService) ListDevices(userID string) ([]models.Device, error) {
	var devices []models.Device
	err := database.DB.Where("user_id = ?", userID).Find(&devices).Error
	return devices, err
}

func (s *DeviceService) GetDevice(userID, deviceID string) (*models.Device, error) {
	var device models.Device
	err := database.DB.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

type CreateDeviceInput struct {
	Name       string `json:"name"`
	ChipType   string `json:"chipType"`
	MACAddress string `json:"macAddress"`
	IPAddress  string `json:"ipAddress"`
	Port       int    `json:"port"`
	APIToken   string `json:"apiToken"`
}

type UpdateDeviceInput struct {
	Name            *string `json:"name"`
	IPAddress       *string `json:"ipAddress"`
	Port            *int    `json:"port"`
	APIToken        *string `json:"apiToken"`
	FirmwareVersion *string `json:"firmwareVersion"`
	IsOnline        *bool   `json:"isOnline"`
}

func (s *DeviceService) CreateDevice(userID string, input CreateDeviceInput) (*models.Device, error) {
	uid, _ := uuid.Parse(userID)
	port := input.Port
	if port == 0 {
		port = 80
	}
	device := models.Device{
		Base:       models.Base{ID: uuid.New()},
		UserID:     uid,
		Name:       input.Name,
		ChipType:   input.ChipType,
		MACAddress: input.MACAddress,
		IPAddress:  input.IPAddress,
		Port:       port,
		APIToken:   input.APIToken,
		Status:     "registered",
	}

	if err := database.DB.Create(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

func (s *DeviceService) UpdateDevice(userID, deviceID string, input UpdateDeviceInput) error {
	updates := map[string]interface{}{}
	if input.Name != nil            { updates["name"] = *input.Name }
	if input.IPAddress != nil       { updates["ip_address"] = *input.IPAddress }
	if input.Port != nil            { updates["port"] = *input.Port }
	if input.APIToken != nil        { updates["api_token"] = *input.APIToken }
	if input.FirmwareVersion != nil { updates["firmware_version"] = *input.FirmwareVersion }
	if input.IsOnline != nil        { updates["is_online"] = *input.IsOnline }
	if len(updates) == 0 {
		return nil
	}
	return database.DB.Model(&models.Device{}).
		Where("id = ? AND user_id = ?", deviceID, userID).
		Updates(updates).Error
}

func (s *DeviceService) DeleteDevice(userID, deviceID string) error {
	return database.DB.Where("id = ? AND user_id = ?", deviceID, userID).
		Delete(&models.Device{}).Error
}
