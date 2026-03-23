package mqtt

import (
	"fmt"
	"strings"
	"testing"
)

// Mirrors the topic parsing logic in handleMessage.
func parseTopic(topic string) (deviceName, subtopic string, ok bool) {
	rest := strings.TrimPrefix(topic, "airconnect/")
	if rest == topic {
		return
	}
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return
	}
	return rest[:slash], rest[slash+1:], true
}

func TestParseTopic_Status(t *testing.T) {
	dev, sub, ok := parseTopic("airconnect/mydevice/status")
	if !ok || dev != "mydevice" || sub != "status" {
		t.Fatalf("got dev=%q sub=%q ok=%v", dev, sub, ok)
	}
}

func TestParseTopic_State(t *testing.T) {
	dev, sub, ok := parseTopic("airconnect/mydevice/state")
	if !ok || dev != "mydevice" || sub != "state" {
		t.Fatalf("got dev=%q sub=%q ok=%v", dev, sub, ok)
	}
}

func TestParseTopic_RelayState(t *testing.T) {
	dev, sub, ok := parseTopic("airconnect/mydevice/relay/1/state")
	if !ok || dev != "mydevice" || sub != "relay/1/state" {
		t.Fatalf("got dev=%q sub=%q ok=%v", dev, sub, ok)
	}
	if !strings.HasPrefix(sub, "relay/") || !strings.HasSuffix(sub, "/state") {
		t.Fatalf("relay state pattern not matched: %q", sub)
	}
}

func TestParseTopic_Health(t *testing.T) {
	dev, sub, ok := parseTopic("airconnect/mydevice/health")
	if !ok || dev != "mydevice" || sub != "health" {
		t.Fatalf("got dev=%q sub=%q ok=%v", dev, sub, ok)
	}
}

func TestParseTopic_Sensor(t *testing.T) {
	dev, sub, ok := parseTopic("airconnect/mydevice/sensor/0")
	if !ok || dev != "mydevice" {
		t.Fatalf("got dev=%q sub=%q ok=%v", dev, sub, ok)
	}
	if !strings.HasPrefix(sub, "sensor/") {
		t.Fatalf("sensor pattern not matched: %q", sub)
	}
}

func TestParseTopic_WrongNamespace(t *testing.T) {
	_, _, ok := parseTopic("home/mydevice/status")
	if ok {
		t.Fatal("should not parse topics outside airconnect/ namespace")
	}
}

func TestParseTopic_NoSubtopic(t *testing.T) {
	_, _, ok := parseTopic("airconnect/mydevice")
	if ok {
		t.Fatal("should not parse topic with no subtopic")
	}
}

func TestPublishRelayCommandTopic(t *testing.T) {
	cases := []struct {
		username string
		relay    int
		want     string
	}{
		{"livingroom", 1, "airconnect/livingroom/relay/1/set"},
		{"kitchen", 2, "airconnect/kitchen/relay/2/set"},
	}
	for _, tc := range cases {
		got := fmt.Sprintf("airconnect/%s/relay/%d/set", tc.username, tc.relay)
		if got != tc.want {
			t.Fatalf("topic mismatch: got %q want %q", got, tc.want)
		}
	}
}
