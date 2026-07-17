package main

import "testing"

func TestForwardingRequiresPrivateNamespace(t *testing.T) {
	if !namespaceIsPrivate("net:[2]", "net:[1]") {
		t.Fatal("different valid namespace identifiers should be private")
	}
	if namespaceIsPrivate("net:[1]", "net:[1]") {
		t.Fatal("the initial network namespace must not be private")
	}
	if !forwardAllowed("127.0.0.1:6053", false) {
		t.Fatal("loopback forwarding should not require namespace isolation")
	}
	if forwardAllowed("192.0.2.10:6053", false) {
		t.Fatal("non-loopback forwarding escaped the host-network guard")
	}
	if !forwardAllowed("192.0.2.10:6053", true) {
		t.Fatal("private namespace should allow a synthetic non-loopback address")
	}
}

func TestNamedScenarios(t *testing.T) {
	for _, name := range []string{"basic-io", "blink"} {
		if _, err := namedScenario(name); err != nil {
			t.Fatalf("scenario %q: %v", name, err)
		}
	}
	if _, err := namedScenario("unknown"); err == nil {
		t.Fatal("unknown scenario unexpectedly succeeded")
	}
}
