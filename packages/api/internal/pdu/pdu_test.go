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

func TestDecodeMultipartUCS2WithSemiOctetAlphaSender(t *testing.T) {
	t.Parallel()

	message, err := Decode("07917710002095F44009D0C1313D6D070008624031610060028C050003B602010412002004120430044800200430043A043A04300443043D044200200442043E043B044C043A043E002004470442043E00200431044B043B00200432044B043F043E043B043D0435043D002004320445043E04340020043D04300020043D043E0432043E043C00200443044104420440043E0439044104420432043500200047006F006F0067")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if message.Sender != "Activ" {
		t.Fatalf("sender = %q", message.Sender)
	}
	if message.Encoding != "ucs2" {
		t.Fatalf("encoding = %q", message.Encoding)
	}
	if message.MultipartRef == nil || *message.MultipartRef != 0xB6 {
		t.Fatalf("multipart ref = %#v", message.MultipartRef)
	}
	if message.MultipartTotal == nil || *message.MultipartTotal != 2 {
		t.Fatalf("multipart total = %#v", message.MultipartTotal)
	}
	if message.MultipartPart == nil || *message.MultipartPart != 1 {
		t.Fatalf("multipart part = %#v", message.MultipartPart)
	}
	expected := "В Ваш аккаунт только что был выполнен вход на новом устройстве Goog"
	if message.Body != expected {
		t.Fatalf("body = %q", message.Body)
	}
}
