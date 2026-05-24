package cyber_guardian

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestCleanProfileLow(t *testing.T) {
	v := New().Inspect(Request{
		UserID:        "u-1",
		KnownDeviceFPs: []string{"fp-pixel"},
		Events: []Event{
			{SuccessfulAuth: true, Lat: 19.05, Lng: 72.85, DeviceFP: "fp-pixel", UnixMillis: 1_700_000_000_000},
		},
	})
	if v.Label != "low" {
		t.Errorf("clean profile should be low; got %s (flags=%v)", v.Label, v.Flags)
	}
}

func TestImpossibleTravel(t *testing.T) {
	// Bombay → New York in 1 hour.
	v := New().Inspect(Request{
		UserID: "u-2",
		KnownDeviceFPs: []string{"fp-1"},
		Events: []Event{
			{SuccessfulAuth: true, Lat: 19.05, Lng: 72.85, DeviceFP: "fp-1", UnixMillis: 1_700_000_000_000},
			{SuccessfulAuth: true, Lat: 40.71, Lng: -74.00, DeviceFP: "fp-1", UnixMillis: 1_700_000_000_000 + (60 * 60 * 1000)},
		},
	})
	if v.Label == "low" {
		t.Errorf("impossible travel should not be low; got %s", v.Label)
	}
	if !strings.Contains(strings.Join(v.Flags, ","), "Impossible travel") {
		t.Errorf("expected impossible-travel flag; got %+v", v.Flags)
	}
}

func TestCredentialStuffing(t *testing.T) {
	events := []Event{}
	for i := 0; i < 6; i++ {
		events = append(events, Event{SuccessfulAuth: false, UnixMillis: int64(i)})
	}
	v := New().Inspect(Request{UserID: "u-3", Events: events})
	if v.Label != "high" && v.Label != "medium" {
		t.Errorf("6 failed attempts should at least be medium; got %s", v.Label)
	}
}

func TestUnknownDevice(t *testing.T) {
	v := New().Inspect(Request{
		UserID:         "u-4",
		KnownDeviceFPs: []string{"fp-1"},
		Events: []Event{
			{SuccessfulAuth: true, DeviceFP: "fp-2", UnixMillis: 1_700_000_000_000},
		},
	})
	hit := false
	for _, f := range v.Flags {
		if strings.Contains(f, "unenrolled device") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected unenrolled-device flag; got %+v", v.Flags)
	}
}

func TestDeviceChurn(t *testing.T) {
	v := New().Inspect(Request{
		UserID:         "u-5",
		KnownDeviceFPs: []string{"fp-1", "fp-2", "fp-3"},
		Events: []Event{
			{SuccessfulAuth: true, DeviceFP: "fp-1", UnixMillis: 1},
			{SuccessfulAuth: true, DeviceFP: "fp-2", UnixMillis: 2},
			{SuccessfulAuth: true, DeviceFP: "fp-3", UnixMillis: 3},
		},
	})
	hit := false
	for _, f := range v.Flags {
		if strings.Contains(f, "device churn") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected device-churn flag; got %+v", v.Flags)
	}
}

func TestHandleMessage(t *testing.T) {
	body, _ := json.Marshal(Request{UserID: "u", Events: []Event{}})
	msg := agent.NewMessage("s", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected dispatch, got %+v", out)
	}
}

func TestDisclaimer(t *testing.T) {
	v := New().Inspect(Request{})
	if v.Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}
