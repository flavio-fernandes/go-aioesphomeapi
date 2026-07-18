package mdns

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
)

const (
	dnsHeaderSize     = 12
	maxDNSMessageSize = 9000
	maxDNSNameJumps   = 16
	typeA             = 1
	typeAAAA          = 28
	typeANY           = 255
	classIN           = 1
	classMask         = 0x7fff
	classCacheFlush   = 0x8000
	responseFlags     = 0x8400
	defaultRecordTTL  = 120
)

var (
	errMalformedMessage = errors.New("malformed mDNS message")
	errNameTooLong      = errors.New("mDNS name is too long")
)

type question struct {
	name   string
	qtype  uint16
	qclass uint16
}

func query(host string) ([]byte, error) {
	name, err := appendName(nil, canonicalName(host))
	if err != nil {
		return nil, err
	}
	message := make([]byte, dnsHeaderSize, dnsHeaderSize+len(name)+4)
	binary.BigEndian.PutUint16(message[4:6], 1)
	message = append(message, name...)
	message = binary.BigEndian.AppendUint16(message, typeA)
	message = binary.BigEndian.AppendUint16(message, classIN)
	return message, nil
}

func response(host string, ip net.IP) ([]byte, error) {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil, errors.New("mDNS test responder requires an IPv4 address")
	}
	name, err := appendName(nil, canonicalName(host))
	if err != nil {
		return nil, err
	}
	message := make([]byte, dnsHeaderSize, dnsHeaderSize+len(name)+14)
	binary.BigEndian.PutUint16(message[2:4], responseFlags)
	binary.BigEndian.PutUint16(message[6:8], 1)
	message = append(message, name...)
	message = binary.BigEndian.AppendUint16(message, typeA)
	message = binary.BigEndian.AppendUint16(message, classIN|classCacheFlush)
	message = binary.BigEndian.AppendUint32(message, defaultRecordTTL)
	message = binary.BigEndian.AppendUint16(message, uint16(net.IPv4len))
	message = append(message, ipv4...)
	return message, nil
}

func questions(message []byte) ([]question, error) {
	if len(message) < dnsHeaderSize {
		return nil, errMalformedMessage
	}
	count := int(binary.BigEndian.Uint16(message[4:6]))
	if count > 64 {
		return nil, errMalformedMessage
	}
	offset := dnsHeaderSize
	result := make([]question, 0, count)
	for range count {
		name, next, err := readName(message, offset)
		if err != nil || next+4 > len(message) {
			return nil, errMalformedMessage
		}
		result = append(result, question{
			name:   name,
			qtype:  binary.BigEndian.Uint16(message[next : next+2]),
			qclass: binary.BigEndian.Uint16(message[next+2 : next+4]),
		})
		offset = next + 4
	}
	return result, nil
}

func answerIP(message []byte, host string) (net.IP, bool) {
	if len(message) < dnsHeaderSize || len(message) > maxDNSMessageSize {
		return nil, false
	}
	if binary.BigEndian.Uint16(message[2:4])&0x8000 == 0 {
		return nil, false
	}
	questionCount := int(binary.BigEndian.Uint16(message[4:6]))
	recordCount := int(binary.BigEndian.Uint16(message[6:8])) + int(binary.BigEndian.Uint16(message[8:10])) + int(binary.BigEndian.Uint16(message[10:12]))
	if questionCount > 64 || recordCount > 256 {
		return nil, false
	}
	offset := dnsHeaderSize
	for range questionCount {
		_, next, err := readName(message, offset)
		if err != nil || next+4 > len(message) {
			return nil, false
		}
		offset = next + 4
	}
	want := canonicalName(host)
	for range recordCount {
		name, next, err := readName(message, offset)
		if err != nil || next+10 > len(message) {
			return nil, false
		}
		recordType := binary.BigEndian.Uint16(message[next : next+2])
		recordClass := binary.BigEndian.Uint16(message[next+2:next+4]) & classMask
		length := int(binary.BigEndian.Uint16(message[next+8 : next+10]))
		data := next + 10
		if data+length > len(message) {
			return nil, false
		}
		if recordClass == classIN && strings.EqualFold(name, want) {
			switch {
			case recordType == typeA && length == net.IPv4len:
				return append(net.IP(nil), message[data:data+length]...), true
			case recordType == typeAAAA && length == net.IPv6len:
				return append(net.IP(nil), message[data:data+length]...), true
			}
		}
		offset = data + length
	}
	return nil, false
}

func appendName(dst []byte, name string) ([]byte, error) {
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return append(dst, 0), nil
	}
	if len(name) > 253 {
		return nil, errNameTooLong
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 {
			return nil, errNameTooLong
		}
		dst = append(dst, byte(len(label)))
		dst = append(dst, label...)
	}
	return append(dst, 0), nil
}

func readName(message []byte, start int) (string, int, error) {
	if start < 0 || start >= len(message) {
		return "", 0, errMalformedMessage
	}
	labels := make([]string, 0, 4)
	offset := start
	next := -1
	jumps := 0
	nameLength := 0
	for {
		if offset >= len(message) {
			return "", 0, errMalformedMessage
		}
		length := int(message[offset])
		switch {
		case length == 0:
			if next < 0 {
				next = offset + 1
			}
			return canonicalName(strings.Join(labels, ".")), next, nil
		case length&0xc0 == 0xc0:
			if offset+1 >= len(message) || jumps >= maxDNSNameJumps {
				return "", 0, errMalformedMessage
			}
			pointer := int(message[offset]&0x3f)<<8 | int(message[offset+1])
			if pointer >= len(message) {
				return "", 0, errMalformedMessage
			}
			if next < 0 {
				next = offset + 2
			}
			offset = pointer
			jumps++
		case length&0xc0 != 0 || length > 63:
			return "", 0, errMalformedMessage
		default:
			offset++
			if offset+length > len(message) {
				return "", 0, errMalformedMessage
			}
			nameLength += length + 1
			if nameLength > 254 {
				return "", 0, errMalformedMessage
			}
			labels = append(labels, string(message[offset:offset+length]))
			offset += length
		}
	}
}

func canonicalName(name string) string {
	return strings.ToLower(strings.TrimSuffix(name, ".")) + "."
}
