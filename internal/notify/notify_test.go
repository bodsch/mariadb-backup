package notify

import (
	"strings"
	"testing"
)

func TestBuildRFC822(t *testing.T) {
	msg := Message{
		From:    "backup@example.com",
		To:      "admin@example.com",
		Subject: "Database Backup at host - 2026-06-12",
		Body:    "line1\nline2",
	}
	got := buildRFC822(msg)
	for _, want := range []string{
		"From: backup@example.com\r\n",
		"To: admin@example.com\r\n",
		"Subject: Database Backup at host - 2026-06-12\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"line1\r\nline2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RFC822 missing %q\nfull:\n%s", want, got)
		}
	}
}

func TestLoginAuth(t *testing.T) {
	a := &loginAuth{username: "user", password: "pass"}
	proto, resp, err := a.Start(nil)
	if err != nil || proto != "LOGIN" || resp != nil {
		t.Fatalf("Start = %q %v %v", proto, resp, err)
	}
	u, err := a.Next([]byte("Username:"), true)
	if err != nil || string(u) != "user" {
		t.Errorf("username challenge: %q %v", u, err)
	}
	p, err := a.Next([]byte("Password:"), true)
	if err != nil || string(p) != "pass" {
		t.Errorf("password challenge: %q %v", p, err)
	}
	if _, err := a.Next([]byte("???"), true); err == nil {
		t.Errorf("unexpected challenge should error")
	}
	if _, err := a.Next(nil, false); err != nil {
		t.Errorf("no-more should not error: %v", err)
	}
}
