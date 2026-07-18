package mdns

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestQueryAndResponse(t *testing.T) {
	request, err := query("ESPHome-Blink.local")
	if err != nil {
		t.Fatalf("build query: %v", err)
	}
	items, err := questions(request)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if len(items) != 1 || items[0].name != "esphome-blink.local." || items[0].qtype != typeA || items[0].qclass != classIN {
		t.Fatalf("unexpected questions: %#v", items)
	}

	reply, err := response("esphome-blink.local", net.IPv4(192, 0, 2, 44))
	if err != nil {
		t.Fatalf("build response: %v", err)
	}
	ip, ok := answerIP(reply, "ESPHOME-BLINK.LOCAL.")
	if !ok || !ip.Equal(net.IPv4(192, 0, 2, 44)) {
		t.Fatalf("got %v, %t", ip, ok)
	}
}

func TestAnswerIPAcceptsCompressedName(t *testing.T) {
	message, err := query("esphome-blink.local")
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint16(message[6:8], 1)
	binary.BigEndian.PutUint16(message[2:4], responseFlags)
	message = append(message, 0xc0, 0x0c)
	message = binary.BigEndian.AppendUint16(message, typeA)
	message = binary.BigEndian.AppendUint16(message, classIN|classCacheFlush)
	message = binary.BigEndian.AppendUint32(message, 120)
	message = binary.BigEndian.AppendUint16(message, 4)
	message = append(message, 203, 0, 113, 7)
	ip, ok := answerIP(message, "esphome-blink.local")
	if !ok || !ip.Equal(net.IPv4(203, 0, 113, 7)) {
		t.Fatalf("got %v, %t", ip, ok)
	}
}

func TestAnswerIPRejectsKnownAnswerInQuery(t *testing.T) {
	message, err := response("esphome-blink.local", net.IPv4(203, 0, 113, 7))
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint16(message[2:4], 0)
	if ip, ok := answerIP(message, "esphome-blink.local"); ok || ip != nil {
		t.Fatalf("accepted a query packet as an answer: %v", ip)
	}
}

func TestMalformedMessagesFailClosed(t *testing.T) {
	loop := make([]byte, dnsHeaderSize+2)
	binary.BigEndian.PutUint16(loop[6:8], 1)
	loop[dnsHeaderSize] = 0xc0
	loop[dnsHeaderSize+1] = dnsHeaderSize

	cases := [][]byte{
		nil,
		make([]byte, dnsHeaderSize-1),
		loop,
		append(make([]byte, dnsHeaderSize), 64),
		make([]byte, maxDNSMessageSize+1),
	}
	for i, message := range cases {
		if ip, ok := answerIP(message, "esphome-blink.local"); ok || ip != nil {
			t.Fatalf("case %d accepted malformed response: %v", i, ip)
		}
	}
}

func FuzzAnswerIP(f *testing.F) {
	valid, _ := response("esphome-blink.local", net.IPv4(127, 0, 0, 1))
	f.Add(valid)
	f.Add([]byte{0xc0, 0x0c})
	f.Fuzz(func(t *testing.T, message []byte) {
		_, _ = answerIP(message, "esphome-blink.local")
	})
}
