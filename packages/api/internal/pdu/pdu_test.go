package pdu

import "testing"

func TestDecodeGSM7(t *testing.T) {
	t.Parallel()

	message, err := Decode("00040A91214365870900004240312143650005E8329BFD06")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if message.Sender != "+1234567890" {
		t.Fatalf("sender = %q", message.Sender)
	}
	if message.Encoding != "gsm7" {
		t.Fatalf("encoding = %q", message.Encoding)
	}
	if message.Body != "hello" {
		t.Fatalf("body = %q", message.Body)
	}
}

func TestDecodeUCS2(t *testing.T) {
	t.Parallel()

	message, err := Decode("00040A9121436587090008424031214365000C041F04400438043204350442")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if message.Encoding != "ucs2" {
		t.Fatalf("encoding = %q", message.Encoding)
	}
	if message.Body != "Привет" {
		t.Fatalf("body = %q", message.Body)
	}
}

func TestDecodeAlphaNumericSender(t *testing.T) {
	t.Parallel()

	message, err := Decode("000406D0C7F7FBCC2E0300004240312143650005E8329BFD06")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if message.Sender != "Google" {
		t.Fatalf("sender = %q", message.Sender)
	}
	if message.Body != "hello" {
		t.Fatalf("body = %q", message.Body)
	}
}

func TestDecodeMultipartUCS2(t *testing.T) {
	t.Parallel()

	message, err := Decode("00440A9121436587090008424031214365000A050003CC0201041F0440")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if message.MultipartRef == nil || *message.MultipartRef != 0xCC {
		t.Fatalf("multipart ref = %#v", message.MultipartRef)
	}
	if message.MultipartTotal == nil || *message.MultipartTotal != 2 {
		t.Fatalf("multipart total = %#v", message.MultipartTotal)
	}
	if message.MultipartPart == nil || *message.MultipartPart != 1 {
		t.Fatalf("multipart part = %#v", message.MultipartPart)
	}
	if message.Body != "Пр" {
		t.Fatalf("body = %q", message.Body)
	}
}
