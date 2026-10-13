package hermes

import (
	"testing"
	"time"
)

func TestSessionLimitsKeepIndependentRecentQuotas(t *testing.T) {
	base := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	list := []SessionSummary{
		{SessionID: "a-old", Project: "/a", LastActive: base},
		{SessionID: "b-old", Project: "/b", LastActive: base},
		{SessionID: "a-new", Project: "/a", LastActive: base.Add(time.Minute)},
		{SessionID: "b-new", Project: "/b", LastActive: base.Add(time.Minute)},
		{SessionID: "a-active-old", Project: "/a", IsGatewayTracked: true, LastActive: base},
		{SessionID: "a-active-new", Project: "/a", IsGatewayTracked: true, LastActive: base.Add(time.Minute)},
		{SessionID: "b-active", Project: "/b", IsGatewayTracked: true, LastActive: base},
	}
	got := applyHermesSessionLimits(list, 1, 1)
	if len(got) != 4 {
		t.Fatalf("got %v", got)
	}
	for _, s := range got {
		if s.SessionID != "a-new" && s.SessionID != "b-new" && s.SessionID != "a-active-new" && s.SessionID != "b-active" {
			t.Fatalf("kept old: %s", s.SessionID)
		}
	}
	if got := applyHermesSessionLimits(list, 1, 0); len(got) != 2 {
		t.Fatal("inactive zero must preserve the independent active quota")
	}
	if got := applyHermesSessionLimits(list, -1, -1); len(got) != 7 {
		t.Fatal("unlimited lost sessions")
	}
}
