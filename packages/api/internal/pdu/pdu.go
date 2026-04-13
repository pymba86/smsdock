package pdu

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"
)

type Message struct {
	Sender         string
	Body           string
	Encoding       string
	Timestamp      *time.Time
	MultipartRef   *int
	MultipartPart  *int
	MultipartTotal *int
}

type concatInfo struct {
	ref   *int
	part  *int
	total *int
}

func Decode(raw string) (Message, error) {
	bytes, err := hex.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return Message{}, fmt.Errorf("decode pdu hex: %w", err)
	}
	if len(bytes) < 2 {
		return Message{}, fmt.Errorf("pdu too short")
	}

	index := 0
	smscLen := int(bytes[index])
	index++
	if len(bytes) < index+smscLen+2 {
		return Message{}, fmt.Errorf("pdu missing smsc or header")
	}
	index += smscLen

	firstOctet := bytes[index]
	index++
	udhi := firstOctet&0x40 != 0

	addressLen := int(bytes[index])
	index++
	addressType := bytes[index]
	index++
	addressIndex := index
	addressSizes := []int{addressByteLength(addressType, addressLen)}
	if isAlphaNumericAddressType(addressType) {
		fallbackSize := (addressLen + 1) / 2
		if fallbackSize != addressSizes[0] {
			addressSizes = append(addressSizes, fallbackSize)
		}
	}

	var firstErr error
	for _, addressSize := range addressSizes {
		message, err := decodeWithAddressSize(bytes, addressIndex, addressType, addressLen, addressSize, udhi)
		if err == nil {
			return message, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}

	if firstErr != nil {
		return Message{}, firstErr
	}
	return Message{}, fmt.Errorf("pdu missing sender metadata")
}

func decodeWithAddressSize(bytes []byte, addressIndex int, addressType byte, addressLen int, addressSize int, udhi bool) (Message, error) {
	index := addressIndex
	if len(bytes) < index+addressSize+9 {
		return Message{}, fmt.Errorf("pdu missing sender metadata")
	}
	addressBytes := bytes[index : index+addressSize]
	index += addressSize

	sender := decodeAddress(addressType, addressLen, addressBytes)

	index++ // pid
	dcs := bytes[index]
	index++

	timestamp, err := decodeTimestamp(bytes[index : index+7])
	if err != nil {
		return Message{}, fmt.Errorf("decode timestamp: %w", err)
	}
	index += 7

	if len(bytes) <= index {
		return Message{}, fmt.Errorf("pdu missing user data length")
	}
	userDataLength := int(bytes[index])
	index++
	if len(bytes) < index {
		return Message{}, fmt.Errorf("pdu invalid user data")
	}
	userData := bytes[index:]

	encoding := detectEncoding(dcs)
	concat := concatInfo{}
	startBit := 0
	userPayload := userData

	if udhi && len(userData) > 0 {
		headerSize := int(userData[0]) + 1
		if headerSize > len(userData) {
			return Message{}, fmt.Errorf("invalid user data header")
		}
		concat = parseConcatInfo(userData[:headerSize])
		if encoding == "gsm7" {
			startBit = headerSize * 8
		} else {
			userPayload = userData[headerSize:]
		}
	}

	body, err := decodeBody(encoding, userDataLength, startBit, userPayload, userData)
	if err != nil {
		return Message{}, fmt.Errorf("decode body: %w", err)
	}

	return Message{
		Sender:         sender,
		Body:           body,
		Encoding:       encoding,
		Timestamp:      timestamp,
		MultipartRef:   concat.ref,
		MultipartPart:  concat.part,
		MultipartTotal: concat.total,
	}, nil
}

func decodeBody(encoding string, userDataLength int, startBit int, userPayload []byte, fullUserData []byte) (string, error) {
	switch encoding {
	case "ucs2":
		return decodeUCS2(userPayload)
	case "gsm7":
		septets := userDataLength
		if startBit > 0 {
			septets -= (startBit + 6) / 7
		}
		if septets < 0 {
			septets = 0
		}
		return decodeGSM7(fullUserData, startBit, septets), nil
	default:
		return strings.ToUpper(hex.EncodeToString(userPayload)), nil
	}
}

func detectEncoding(dcs byte) string {
	switch (dcs >> 2) & 0x03 {
	case 0x00:
		return "gsm7"
	case 0x02:
		return "ucs2"
	default:
		return "8bit"
	}
}

func addressByteLength(addressType byte, addressLength int) int {
	if isAlphaNumericAddressType(addressType) {
		return (addressLength*7 + 7) / 8
	}
	return (addressLength + 1) / 2
}

func decodeAddress(addressType byte, addressLength int, address []byte) string {
	if isAlphaNumericAddressType(addressType) {
		septets := min(addressLength, (len(address)*8)/7)
		return strings.TrimRight(decodeGSM7(address, 0, septets), "@")
	}
	digits := decodeSemiOctet(address)
	if len(digits) > addressLength {
		digits = digits[:addressLength]
	}
	if addressType&0x90 == 0x90 {
		return "+" + digits
	}
	return digits
}

func isAlphaNumericAddressType(addressType byte) bool {
	return addressType&0x70 == 0x50
}

func decodeSemiOctet(input []byte) string {
	var builder strings.Builder
	for _, value := range input {
		first := value & 0x0f
		second := (value & 0xf0) >> 4
		builder.WriteByte(nibbleToChar(first))
		if second != 0x0f {
			builder.WriteByte(nibbleToChar(second))
		}
	}
	return builder.String()
}

func nibbleToChar(value byte) byte {
	if value <= 9 {
		return '0' + value
	}
	return 'F'
}

func decodeTimestamp(input []byte) (*time.Time, error) {
	if len(input) != 7 {
		return nil, fmt.Errorf("timestamp must be 7 bytes")
	}

	for _, value := range input[:6] {
		if !isBCD(value) {
			return nil, fmt.Errorf("timestamp contains invalid bcd")
		}
	}
	if !isBCD(input[6] & 0x7f) {
		return nil, fmt.Errorf("timestamp timezone contains invalid bcd")
	}

	year := 2000 + swappedBCD(input[0])
	month := swappedBCD(input[1])
	day := swappedBCD(input[2])
	hour := swappedBCD(input[3])
	minute := swappedBCD(input[4])
	second := swappedBCD(input[5])
	switch {
	case month < 1 || month > 12:
		return nil, fmt.Errorf("timestamp month out of range")
	case day < 1 || day > 31:
		return nil, fmt.Errorf("timestamp day out of range")
	case hour > 23:
		return nil, fmt.Errorf("timestamp hour out of range")
	case minute > 59:
		return nil, fmt.Errorf("timestamp minute out of range")
	case second > 59:
		return nil, fmt.Errorf("timestamp second out of range")
	}

	tz := input[6]
	offsetQuarters := swappedBCD(tz & 0x7f)
	offsetMinutes := offsetQuarters * 15
	if tz&0x80 != 0 {
		offsetMinutes = -offsetMinutes
	}

	location := time.FixedZone("sms", offsetMinutes*60)
	timestamp := time.Date(year, time.Month(month), day, hour, minute, second, 0, location).UTC()
	return &timestamp, nil
}

func swappedBCD(value byte) int {
	return int(value&0x0f)*10 + int((value&0xf0)>>4)
}

func isBCD(value byte) bool {
	return value&0x0f <= 9 && (value>>4)&0x0f <= 9
}

func parseConcatInfo(header []byte) concatInfo {
	info := concatInfo{}
	if len(header) < 2 {
		return info
	}

	for index := 1; index+1 < len(header); {
		identifier := header[index]
		length := int(header[index+1])
		index += 2
		if index+length > len(header) {
			break
		}
		payload := header[index : index+length]
		index += length

		switch {
		case identifier == 0x00 && len(payload) == 3:
			ref := int(payload[0])
			total := int(payload[1])
			part := int(payload[2])
			info.ref = &ref
			info.total = &total
			info.part = &part
		case identifier == 0x08 && len(payload) == 4:
			ref := int(payload[0])<<8 | int(payload[1])
			total := int(payload[2])
			part := int(payload[3])
			info.ref = &ref
			info.total = &total
			info.part = &part
		}
	}

	return info
}

func decodeGSM7(data []byte, startBit, septets int) string {
	if septets <= 0 {
		return ""
	}

	var builder strings.Builder
	for index := 0; index < septets; index++ {
		offset := startBit + index*7
		byteIndex := offset / 8
		shift := offset % 8

		value := uint16(0)
		if byteIndex < len(data) {
			value = uint16(data[byteIndex] >> shift)
		}
		if shift > 1 && byteIndex+1 < len(data) {
			value |= uint16(data[byteIndex+1]) << uint16(8-shift)
		}

		builder.WriteRune(gsm7DecodeTable[value&0x7f])
	}

	return builder.String()
}

func decodeUCS2(data []byte) (string, error) {
	if len(data)%2 != 0 {
		return "", fmt.Errorf("ucs2 payload has odd length")
	}

	codeUnits := make([]uint16, 0, len(data)/2)
	for index := 0; index < len(data); index += 2 {
		codeUnits = append(codeUnits, uint16(data[index])<<8|uint16(data[index+1]))
	}

	return string(utf16.Decode(codeUnits)), nil
}

var gsm7DecodeTable = map[uint16]rune{
	0x00: '@',
	0x01: '£',
	0x02: '$',
	0x03: '¥',
	0x04: 'è',
	0x05: 'é',
	0x06: 'ù',
	0x07: 'ì',
	0x08: 'ò',
	0x09: 'Ç',
	0x0A: '\n',
	0x0B: 'Ø',
	0x0C: 'ø',
	0x0D: '\r',
	0x0E: 'Å',
	0x0F: 'å',
	0x10: 'Δ',
	0x11: '_',
	0x12: 'Φ',
	0x13: 'Γ',
	0x14: 'Λ',
	0x15: 'Ω',
	0x16: 'Π',
	0x17: 'Ψ',
	0x18: 'Σ',
	0x19: 'Θ',
	0x1A: 'Ξ',
	0x1B: ' ',
	0x1C: 'Æ',
	0x1D: 'æ',
	0x1E: 'ß',
	0x1F: 'É',
	0x20: ' ',
	0x21: '!',
	0x22: '"',
	0x23: '#',
	0x24: '¤',
	0x25: '%',
	0x26: '&',
	0x27: '\'',
	0x28: '(',
	0x29: ')',
	0x2A: '*',
	0x2B: '+',
	0x2C: ',',
	0x2D: '-',
	0x2E: '.',
	0x2F: '/',
	0x30: '0',
	0x31: '1',
	0x32: '2',
	0x33: '3',
	0x34: '4',
	0x35: '5',
	0x36: '6',
	0x37: '7',
	0x38: '8',
	0x39: '9',
	0x3A: ':',
	0x3B: ';',
	0x3C: '<',
	0x3D: '=',
	0x3E: '>',
	0x3F: '?',
	0x40: '¡',
	0x41: 'A',
	0x42: 'B',
	0x43: 'C',
	0x44: 'D',
	0x45: 'E',
	0x46: 'F',
	0x47: 'G',
	0x48: 'H',
	0x49: 'I',
	0x4A: 'J',
	0x4B: 'K',
	0x4C: 'L',
	0x4D: 'M',
	0x4E: 'N',
	0x4F: 'O',
	0x50: 'P',
	0x51: 'Q',
	0x52: 'R',
	0x53: 'S',
	0x54: 'T',
	0x55: 'U',
	0x56: 'V',
	0x57: 'W',
	0x58: 'X',
	0x59: 'Y',
	0x5A: 'Z',
	0x5B: 'Ä',
	0x5C: 'Ö',
	0x5D: 'Ñ',
	0x5E: 'Ü',
	0x5F: '§',
	0x60: '¿',
	0x61: 'a',
	0x62: 'b',
	0x63: 'c',
	0x64: 'd',
	0x65: 'e',
	0x66: 'f',
	0x67: 'g',
	0x68: 'h',
	0x69: 'i',
	0x6A: 'j',
	0x6B: 'k',
	0x6C: 'l',
	0x6D: 'm',
	0x6E: 'n',
	0x6F: 'o',
	0x70: 'p',
	0x71: 'q',
	0x72: 'r',
	0x73: 's',
	0x74: 't',
	0x75: 'u',
	0x76: 'v',
	0x77: 'w',
	0x78: 'x',
	0x79: 'y',
	0x7A: 'z',
	0x7B: 'ä',
	0x7C: 'ö',
	0x7D: 'ñ',
	0x7E: 'ü',
	0x7F: 'à',
}
