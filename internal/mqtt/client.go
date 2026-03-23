package mqtt

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/airconnect/backend/internal/database"
	"github.com/airconnect/backend/internal/models"
	"github.com/airconnect/backend/internal/ws"
	"github.com/google/uuid"
	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

var Client pahomqtt.Client

// Callbacks set by main.go to avoid circular imports with services package
var OnDeviceOnline func(deviceID uuid.UUID)
var OnDeviceOffline func(deviceID uuid.UUID)

type Config struct {
	BrokerURL string
	Username  string
	Password  string
}

func Connect(cfg Config) {
	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(fmt.Sprintf("airconnect-backend-%d", time.Now().UnixMilli())).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetOnConnectHandler(onConnect).
		SetConnectionLostHandler(onConnectionLost)

	// Enable TLS for ssl:// or tls:// broker URLs (e.g. HiveMQ Cloud)
	if strings.HasPrefix(cfg.BrokerURL, "ssl://") || strings.HasPrefix(cfg.BrokerURL, "tls://") {
		opts.SetTLSConfig(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})
	}

	Client = pahomqtt.NewClient(opts)
	token := Client.Connect()
	if token.Wait() && token.Error() != nil {
		log.Printf("MQTT connection failed: %v (will retry)", token.Error())
	}
}

func onConnect(client pahomqtt.Client) {
	log.Println("MQTT connected to broker")
	client.Subscribe("airconnect/#", 1, handleMessage)
	log.Println("MQTT subscribed to airconnect/#")
}

func onConnectionLost(client pahomqtt.Client, err error) {
	log.Printf("MQTT connection lost: %v", err)
}

// handleMessage routes incoming MQTT messages by topic structure:
//
//	airconnect/{deviceName}/status        → online/offline, update DB
//	airconnect/{deviceName}/state         → full relay snapshot, broadcast to app
//	airconnect/{deviceName}/relay/N/state → single relay change, broadcast to app
//	airconnect/{deviceName}/health        → health data, broadcast to app
//	airconnect/{deviceName}/sensor/N      → sensor reading
func handleMessage(_ pahomqtt.Client, msg pahomqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())

	// Strip "airconnect/" prefix → "deviceName/subtopic"
	rest := strings.TrimPrefix(topic, "airconnect/")
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return
	}
	deviceName := rest[:slash]
	subtopic := rest[slash+1:]

	switch subtopic {
	case "status":
		go handleStatus(deviceName, payload)
	case "state":
		go handleState(deviceName, payload)
	case "health":
		go handleHealth(deviceName, payload)
	case "schedules/count":
		go handleScheduleCount(deviceName, payload)
	case "ota/progress":
		go handleOTAProgress(deviceName, payload)
	default:
		if strings.HasPrefix(subtopic, "relay/") && strings.HasSuffix(subtopic, "/state") {
			go handleRelayState(deviceName, subtopic, payload)
		} else if strings.HasPrefix(subtopic, "sensor/") {
			go handleSensor(deviceName, subtopic, payload)
		}
	}
}

// handleStatus processes online/offline messages and updates the DB.
func handleStatus(deviceName, payload string) {
	var s struct {
		State   string `json:"state"`
		MAC     string `json:"mac"`
		IP      string `json:"ip"`
		Version string `json:"version"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		return
	}

	isOnline := s.State == "online"
	now := time.Now()

	// Find device by MAC (most reliable) or by mqtt_username matching deviceName
	var device models.Device
	result := database.DB.Where("mac_address = ?", s.MAC).
		Or("mqtt_username = ?", deviceName).
		First(&device)
	if result.Error != nil {
		return // unregistered device — ignore
	}

	updates := map[string]interface{}{
		"is_online": isOnline,
		"last_seen": now,
	}
	if isOnline {
		if s.IP != "" {
			updates["ip_address"] = s.IP
		}
		if s.Version != "" {
			updates["firmware_version"] = s.Version
		}
		// Auto-populate mqtt_username from deviceName if not set
		if device.MQTTUsername == "" {
			updates["mqtt_username"] = deviceName
		}
	}
	database.DB.Model(&device).Updates(updates)

	// Broadcast to app
	msg, _ := json.Marshal(map[string]interface{}{
		"event":    "device_status",
		"deviceId": device.ID,
		"isOnline": isOnline,
		"ip":       s.IP,
	})
	ws.Default.Broadcast(device.UserID.String(), msg)

	log.Printf("[MQTT] Device %s (%s) is %s", device.Name, deviceName, s.State)

	if isOnline && OnDeviceOnline != nil {
		go OnDeviceOnline(device.ID)
	} else if !isOnline && OnDeviceOffline != nil {
		go OnDeviceOffline(device.ID)
	}
}

// handleState processes a full relay state snapshot.
func handleState(deviceName, payload string) {
	var s struct {
		Relay1 bool `json:"relay1"`
		Relay2 bool `json:"relay2"`
	}
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		return
	}

	var device models.Device
	if err := database.DB.Where("mqtt_username = ?", deviceName).First(&device).Error; err != nil {
		return
	}

	msg, _ := json.Marshal(map[string]interface{}{
		"event":    "device_state",
		"deviceId": device.ID,
		"relay1":   s.Relay1,
		"relay2":   s.Relay2,
	})
	ws.Default.Broadcast(device.UserID.String(), msg)
}

// handleRelayState processes a single relay ON/OFF change.
func handleRelayState(deviceName, subtopic, payload string) {
	// subtopic: "relay/1/state" or "relay/2/state"
	parts := strings.Split(subtopic, "/")
	if len(parts) != 3 {
		return
	}
	relayNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}

	var device models.Device
	if err := database.DB.Where("mqtt_username = ?", deviceName).First(&device).Error; err != nil {
		return
	}

	state := strings.EqualFold(payload, "ON")
	msg, _ := json.Marshal(map[string]interface{}{
		"event":    "relay_state",
		"deviceId": device.ID,
		"relay":    relayNum,
		"state":    state,
	})
	ws.Default.Broadcast(device.UserID.String(), msg)
}

// handleHealth broadcasts health data to the app.
func handleHealth(deviceName, payload string) {
	var h map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &h); err != nil {
		return
	}

	var device models.Device
	if err := database.DB.Where("mqtt_username = ?", deviceName).First(&device).Error; err != nil {
		return
	}

	h["event"] = "device_health"
	h["deviceId"] = device.ID
	msg, _ := json.Marshal(h)
	ws.Default.Broadcast(device.UserID.String(), msg)
}

// handleSensor stores sensor readings.
func handleSensor(deviceName, subtopic, payload string) {
	parts := strings.Split(subtopic, "/")
	if len(parts) < 2 {
		return
	}

	var device models.Device
	if err := database.DB.Where("mqtt_username = ?", deviceName).First(&device).Error; err != nil {
		return
	}

	data, _ := json.Marshal(map[string]interface{}{
		"slot":  parts[1],
		"value": payload,
	})
	record := models.TelemetryRecord{
		DeviceID:  device.ID,
		Data:      data,
		Timestamp: time.Now(),
	}
	database.DB.Create(&record)
}

// handleScheduleCount compares device schedule count with backend DB.
// If mismatched, triggers a full resync.
func handleScheduleCount(deviceName, payload string) {
	var sc struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(payload), &sc); err != nil {
		return
	}

	var device models.Device
	if err := database.DB.Where("mqtt_username = ?", deviceName).
		Or("name = ?", deviceName).First(&device).Error; err != nil {
		return
	}

	var dbCount int64
	database.DB.Model(&models.Schedule{}).
		Where("device_id = ? AND enabled = ?", device.ID, true).
		Count(&dbCount)

	if int64(sc.Count) != dbCount {
		log.Printf("[MQTT] Schedule count mismatch for %s: device=%d, db=%d — resyncing",
			deviceName, sc.Count, dbCount)
		if OnDeviceOnline != nil {
			go OnDeviceOnline(device.ID)
		}
	}
}

// handleOTAProgress relays OTA flash progress from device → app via WebSocket.
func handleOTAProgress(deviceName, payload string) {
	var p struct {
		Percent int    `json:"percent"`
		Stage   string `json:"stage"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return
	}

	var device models.Device
	if err := database.DB.Where("mqtt_username = ?", deviceName).First(&device).Error; err != nil {
		return
	}

	msg, _ := json.Marshal(map[string]interface{}{
		"event":    "ota_progress",
		"deviceId": device.ID,
		"percent":  p.Percent,
		"stage":    p.Stage,
		"message":  p.Message,
	})
	ws.Default.Broadcast(device.UserID.String(), msg)
}

// PublishRelayCommand sends a relay command to the device via MQTT.
// state: "ON", "OFF", or "TOGGLE"
func PublishRelayCommand(mqttUsername string, relay int, state string) error {
	topic := fmt.Sprintf("airconnect/%s/relay/%d/set", mqttUsername, relay)
	return Publish(topic, state, false)
}

func Publish(topic, payload string, retained bool) error {
	if Client == nil || !Client.IsConnected() {
		return fmt.Errorf("mqtt not connected")
	}
	token := Client.Publish(topic, 1, retained, payload)
	token.Wait()
	return token.Error()
}

// PublishScheduleCommand sends a schedule add/clear command via MQTT cmd topic.
func PublishScheduleCommand(mqttUsername string, payload string) error {
	topic := fmt.Sprintf("airconnect/%s/cmd", mqttUsername)
	return Publish(topic, payload, false)
}

// SendCommand kept for backwards compatibility
func SendCommand(deviceMAC, command, payload string) error {
	topic := fmt.Sprintf("airconnect/%s/command", deviceMAC)
	return Publish(topic, payload, false)
}

// PublishOTACommand tells a device to download and flash firmware from a URL.
func PublishOTACommand(mqttUsername, firmwareURL string) error {
	payload := fmt.Sprintf(`{"cmd":"ota_update","url":"%s"}`, firmwareURL)
	topic := fmt.Sprintf("airconnect/%s/cmd", mqttUsername)
	return Publish(topic, payload, false)
}
