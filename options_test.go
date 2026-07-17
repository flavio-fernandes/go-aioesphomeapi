package aioesphomeapi

import "testing"

func TestIsMDNSHost(t *testing.T) {
	tests := map[string]bool{
		"esphome-blink.local":  true,
		"ESPHOME-BLINK.LOCAL.": true,
		"not-local.example":    false,
		"almostlocal":          false,
		"local":                false,
	}
	for host, want := range tests {
		if got := isMDNSHost(host); got != want {
			t.Errorf("isMDNSHost(%q) = %t, want %t", host, got, want)
		}
	}
}
