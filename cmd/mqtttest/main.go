// One-off MQTT smoke test — publish sample device messages, then exit.
package main

import (
	"fmt"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

func pub(c pahomqtt.Client, topic, payload string) {
	tok := c.Publish(topic, 1, false, payload)
	tok.Wait()
	if err := tok.Error(); err != nil {
		fmt.Printf("FAIL  %-50s %v\n", topic, err)
	} else {
		fmt.Printf("OK    %s\n", topic)
	}
}

func main() {
	opts := pahomqtt.NewClientOptions().
		AddBroker("tcp://localhost:1883").
		SetClientID("airconnect-smoketest").
		SetConnectTimeout(5 * time.Second)

	c := pahomqtt.NewClient(opts)
	if tok := c.Connect(); tok.Wait() && tok.Error() != nil {
		fmt.Println("MQTT connect failed:", tok.Error())
		return
	}
	defer c.Disconnect(250)
	fmt.Println("Connected to broker")

	// Status message (device comes online)
	pub(c, "airconnect/testdevice/status",
		`{"state":"online","name":"testdevice","mac":"AA:BB:CC:DD:EE:01","ip":"192.168.1.99","version":"2.0.0"}`)

	// Full relay state snapshot
	pub(c, "airconnect/testdevice/state",
		`{"relay1":true,"relay2":false,"uptime":120}`)

	// Individual relay change
	pub(c, "airconnect/testdevice/relay/1/state", "ON")
	pub(c, "airconnect/testdevice/relay/2/state", "OFF")

	// Health tick
	pub(c, "airconnect/testdevice/health",
		`{"uptime":120,"freeHeap":182000,"rssi":-58,"wifiConnected":true,"mqttConnected":true}`)

	// Sensor reading
	pub(c, "airconnect/testdevice/sensor/0", "23.50")

	fmt.Println("\nAll messages published.")
}
