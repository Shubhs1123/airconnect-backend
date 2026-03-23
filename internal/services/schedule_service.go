package services

import (
	"fmt"
	"time"

	"github.com/airconnect/backend/internal/database"
	"github.com/airconnect/backend/internal/models"
	"github.com/airconnect/backend/internal/mqtt"
	"github.com/google/uuid"
)

// SyncAllSchedulesToDevice clears the device schedules and re-pushes ALL
// enabled schedules for that device via MQTT. Called on:
//   - schedule create/update (from handler)
//   - device reconnect (from MQTT status handler)
//   - schedule count mismatch (from MQTT count handler)
func SyncAllSchedulesToDevice(deviceID uuid.UUID) {
	var device models.Device
	if err := database.DB.Where("id = ?", deviceID).First(&device).Error; err != nil {
		fmt.Printf("[schedule-sync] Device not found for ID %s: %v\n", deviceID.String(), err)
		return
	}

	var schedules []models.Schedule
	database.DB.Where("device_id = ? AND enabled = ?", deviceID, true).Find(&schedules)

	mqttName := device.MQTTUsername
	if mqttName == "" {
		mqttName = device.Name
	}
	if mqttName == "" {
		fmt.Printf("[schedule-sync] Device %s has no MQTT name\n", deviceID.String())
		return
	}

	// Clear existing on device
	if err := mqtt.PublishScheduleCommand(mqttName, `{"cmd":"schedule_clear"}`); err != nil {
		fmt.Printf("[schedule-sync] MQTT publish failed for %s: %v\n", mqttName, err)
		return
	}
	time.Sleep(100 * time.Millisecond)

	// Push each schedule
	for _, s := range schedules {
		state := 0
		if s.State {
			state = 1
		}

		var payload string
		if s.GPIO >= 0 {
			payload = fmt.Sprintf(
				`{"cmd":"schedule_add","hour":%d,"minute":%d,"days":%d,"gpio":%d,"state":%d,"enabled":true}`,
				s.Hour, s.Minute, s.Days, s.GPIO, state,
			)
		} else {
			payload = fmt.Sprintf(
				`{"cmd":"schedule_add","hour":%d,"minute":%d,"days":%d,"relay":%d,"state":%d,"enabled":true}`,
				s.Hour, s.Minute, s.Days, s.Relay, state,
			)
		}
		if err := mqtt.PublishScheduleCommand(mqttName, payload); err != nil {
			fmt.Printf("[schedule-sync] MQTT failed mid-push for %s: %v\n", mqttName, err)
			return
		}
		database.DB.Model(&s).Update("synced", true)
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("[schedule-sync] %d schedules pushed via MQTT to %s\n", len(schedules), mqttName)
}

// MarkSchedulesUnsynced sets synced=false for all schedules on a device.
// Called when device goes offline so we know to re-push on reconnect.
func MarkSchedulesUnsynced(deviceID uuid.UUID) {
	database.DB.Model(&models.Schedule{}).
		Where("device_id = ?", deviceID).
		Update("synced", false)
}
