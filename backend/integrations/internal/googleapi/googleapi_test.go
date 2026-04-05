package googleapi

import (
	"strings"
	"testing"
)

func TestCompactEmailsDeduplicatesAndTrims(t *testing.T) {
	got := compactEmails([]string{" A@x.com ", "a@x.com", "", "b@x.com"})
	if len(got) != 2 {
		t.Fatalf("expected 2 unique addresses, got %d", len(got))
	}
}

func TestHasHeaderInjection(t *testing.T) {
	if !hasHeaderInjection("bad\nvalue") {
		t.Fatalf("expected newline to be detected")
	}
	if hasHeaderInjection("clean value") {
		t.Fatalf("did not expect clean value to be flagged")
	}
}

func TestBuildMimeMessageContentModes(t *testing.T) {
	plain := buildMimeMessage([]string{"x@y.com"}, nil, nil, "Subject", "hello", "")
	if !strings.Contains(plain, "Content-Type: text/plain") {
		t.Fatalf("expected text/plain message")
	}
	mixed := buildMimeMessage([]string{"x@y.com"}, nil, nil, "Subject", "hello", "<b>hi</b>")
	if !strings.Contains(mixed, "multipart/alternative") {
		t.Fatalf("expected multipart message")
	}
}

func TestSendEmailValidation(t *testing.T) {
	_, err := SendEmail(nil, SendMailRequest{Subject: "", To: []string{"a@x.com"}, BodyText: "x"})
	if err == nil {
		t.Fatalf("expected subject validation error")
	}
	_, err = SendEmail(nil, SendMailRequest{Subject: "ok", To: nil, BodyText: "x"})
	if err == nil {
		t.Fatalf("expected recipient validation error")
	}
	_, err = SendEmail(nil, SendMailRequest{Subject: "ok", To: []string{"a@x.com"}})
	if err == nil {
		t.Fatalf("expected body validation error")
	}
}

func TestURLEncode(t *testing.T) {
	encoded := urlEncode("a b+c?d")
	if strings.Contains(encoded, " ") || !strings.Contains(encoded, "%20") {
		t.Fatalf("expected encoded spaces, got %q", encoded)
	}
}
