package mdns

import (
	"net"
	"testing"
)

// TestAnswerIPAllocationBudget bounds the parse cost of one well-formed
// multicast DNS answer. The ceiling is several times the measured steady-state
// cost so platform variation never trips it; it exists to catch accidental
// allocation growth on this untrusted-input path.
func TestAnswerIPAllocationBudget(t *testing.T) {
	message, err := response("esphome-blink.local", net.IPv4(203, 0, 113, 7))
	if err != nil {
		t.Fatalf("build response: %v", err)
	}
	average := testing.AllocsPerRun(200, func() {
		ip, ok := answerIP(message, "esphome-blink.local")
		if !ok || ip == nil {
			t.Fatalf("answerIP = %v, %t", ip, ok)
		}
	})
	t.Logf("measured %.1f allocations per message", average)
	if average > 16 {
		t.Fatalf("answerIP allocations per message = %.1f, budget 16", average)
	}
}
