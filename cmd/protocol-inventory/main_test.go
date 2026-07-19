package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func repositoryPath(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

func TestInventoryIsCompleteAndCurrent(t *testing.T) {
	value, err := buildInventory(
		repositoryPath("protocol", "inventory.annotations.json"),
		repositoryPath("protocol", "upstream.lock.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(value.Messages), 148; got != want {
		t.Fatalf("message count = %d, want %d", got, want)
	}
	ids := make(map[uint32]string, len(value.Messages))
	names := make(map[string]entry, len(value.Messages))
	m1Count := 0
	implementedCount := 0
	for _, message := range value.Messages {
		if previous, exists := ids[message.ID]; exists {
			t.Fatalf("duplicate id %d for %s and %s", message.ID, previous, message.Name)
		}
		if _, exists := names[message.Name]; exists {
			t.Fatalf("duplicate name %s", message.Name)
		}
		ids[message.ID] = message.Name
		names[message.Name] = message
		if message.VersionGate.Status == "" || message.VersionGate.KnownAt == "" || message.FeatureGate == "" || message.EntityFamily == "" || message.ReferenceParity == "" || message.PublicBehavior == "" || message.Notes == "" {
			t.Fatalf("message %s has an incomplete compatibility view", message.Name)
		}
		if message.Evidence.Known == nil || message.Evidence.Typed == nil || message.Evidence.Simulated == nil || message.Evidence.MGMT == nil || message.Evidence.Hardware == nil || message.Evidence.Production == nil {
			t.Fatalf("message %s omits an evidence level", message.Name)
		}
		if len(message.Evidence.Known) == 0 {
			t.Fatalf("message %s lacks pinned upstream evidence", message.Name)
		}
		if message.Milestone == "M1" {
			m1Count++
			if message.ReferenceParity == "not_assessed" {
				t.Fatalf("M1 message %s is not fully classified", message.Name)
			}
		}
		if message.PublicBehavior != "generated_only" {
			implementedCount++
			if len(message.Evidence.Typed) == 0 || len(message.Evidence.Simulated) == 0 {
				t.Fatalf("implemented message %s lacks typed or simulated evidence", message.Name)
			}
		}
	}
	if got, want := m1Count, 33; got != want {
		t.Fatalf("M1 message count = %d, want %d", got, want)
	}
	if got, want := implementedCount, 33; got != want {
		t.Fatalf("implemented message count = %d, want %d", got, want)
	}
	expectedM1 := []string{
		"HelloRequest", "HelloResponse", "DisconnectRequest", "DisconnectResponse",
		"PingRequest", "PingResponse", "ListEntitiesRequest", "ListEntitiesDoneResponse",
		"DeviceInfoRequest", "DeviceInfoResponse",
		"SubscribeStatesRequest", "ListEntitiesBinarySensorResponse", "BinarySensorStateResponse",
		"ListEntitiesSensorResponse", "SensorStateResponse", "ListEntitiesTextSensorResponse",
		"TextSensorStateResponse", "ListEntitiesSwitchResponse", "SwitchStateResponse",
		"SwitchCommandRequest", "ListEntitiesNumberResponse", "NumberStateResponse",
		"NumberCommandRequest", "ListEntitiesButtonResponse", "ButtonCommandRequest",
		"ListEntitiesFanResponse", "FanStateResponse", "FanCommandRequest",
		"ListEntitiesLightResponse", "LightStateResponse", "LightCommandRequest",
		"SubscribeLogsRequest", "SubscribeLogsResponse",
	}
	for _, name := range expectedM1 {
		message, exists := names[name]
		if !exists || message.Milestone != "M1" {
			t.Fatalf("required M1 message %s is not accounted for", name)
		}
	}

	assertEvidence := func(name string, mgmt, hardware bool) {
		t.Helper()
		message, exists := names[name]
		if !exists {
			t.Fatalf("missing message %s", name)
		}
		if got := len(message.Evidence.MGMT) > 0; got != mgmt {
			t.Fatalf("%s MGMT evidence = %t, want %t", name, got, mgmt)
		}
		if got := len(message.Evidence.Hardware) > 0; got != hardware {
			t.Fatalf("%s hardware evidence = %t, want %t", name, got, hardware)
		}
	}
	assertEvidence("BinarySensorStateResponse", true, true)
	assertEvidence("FanCommandRequest", true, false)
	assertEvidence("TextSensorStateResponse", false, false)
	assertEvidence("ButtonCommandRequest", false, false)
	assertEvidence("CoverCommandRequest", false, false)

	if value.UnknownValues.MessageIDs.Status != "verified" || value.UnknownValues.MessageIDs.TestPlan == "" {
		t.Fatal("unknown message-ID behavior must have verified evidence and a test plan")
	}
	if value.UnknownValues.EnumValues.Status != "planned" || value.UnknownValues.EnumValues.TestPlan == "" {
		t.Fatal("unknown enum behavior must remain an explicit test plan until evidenced")
	}

	generated, err := render(value)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(repositoryPath("protocol", "inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, committed) {
		t.Fatal("protocol/inventory.json is stale")
	}
}

func TestFeatureFamilyAcceptsKnownGates(t *testing.T) {
	for _, test := range []struct {
		ifdef string
		want  string
	}{
		{"", "protocol"},
		{"USE_FAN", "fan"},
		{"USE_BLUETOOTH_PROXY", "bluetooth_proxy"},
		{"USE_IR_RF || USE_RADIO_FREQUENCY", "infrared_radio_frequency"},
	} {
		got, err := featureFamily(test.ifdef)
		if err != nil {
			t.Fatalf("featureFamily(%q): %v", test.ifdef, err)
		}
		if got != test.want {
			t.Fatalf("featureFamily(%q) = %q, want %q", test.ifdef, got, test.want)
		}
	}
}

func TestFeatureFamilyRejectsUnmappedExpressions(t *testing.T) {
	for _, ifdef := range []string{
		"USE_A || USE_B",
		"USE_A && USE_B",
		"defined(USE_A)",
		"USE_",
	} {
		family, err := featureFamily(ifdef)
		if err == nil {
			t.Fatalf("featureFamily(%q) = %q, want a validation error", ifdef, family)
		}
		if !strings.Contains(err.Error(), strconv.Quote(ifdef)) {
			t.Fatalf("error for %q does not identify the offending expression: %v", ifdef, err)
		}
	}
	_, err := featureFamily("USE_A || USE_B")
	if err == nil || !strings.Contains(err.Error(), strconv.Quote("a || use_b")) {
		t.Fatalf("error does not report the invalid derived value: %v", err)
	}
}
