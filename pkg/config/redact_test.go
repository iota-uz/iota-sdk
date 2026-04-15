package config

import (
	"strings"
	"testing"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password" secret:"true"`
	Token    string `json:"token"    secret:"true"`
}

type dbConfig struct {
	Host  string      `json:"host"`
	Creds credentials `json:"creds"`
}

type withSlice struct {
	Items []credentials `json:"items"`
}

func TestRedact_SecretFieldMasked(t *testing.T) {
	t.Parallel()

	c := credentials{
		Username: "alice",
		Password: "s3cr3t",
		Token:    "tok123",
	}
	out := Redact(c)
	if !strings.Contains(out, `"username": "alice"`) {
		t.Errorf("non-secret field should be visible, got:\n%s", out)
	}
	if !strings.Contains(out, `"password": "***"`) {
		t.Errorf("secret password should be masked, got:\n%s", out)
	}
	if !strings.Contains(out, `"token": "***"`) {
		t.Errorf("secret token should be masked, got:\n%s", out)
	}
}

func TestRedact_ZeroValuedSecretIsEmpty(t *testing.T) {
	t.Parallel()

	c := credentials{Username: "bob"} // Password and Token are zero
	out := Redact(c)
	if !strings.Contains(out, `"password": ""`) {
		t.Errorf("zero-valued secret should emit empty string, got:\n%s", out)
	}
	if !strings.Contains(out, `"token": ""`) {
		t.Errorf("zero-valued secret should emit empty string, got:\n%s", out)
	}
}

func TestRedact_NonSecretPreserved(t *testing.T) {
	t.Parallel()

	c := credentials{Username: "charlie", Password: "pw"}
	out := Redact(c)
	if !strings.Contains(out, "charlie") {
		t.Errorf("non-secret username should appear, got:\n%s", out)
	}
}

func TestRedact_NestedStructWalked(t *testing.T) {
	t.Parallel()

	cfg := dbConfig{
		Host: "db.local",
		Creds: credentials{
			Username: "dbuser",
			Password: "dbpass",
		},
	}
	out := Redact(cfg)
	if !strings.Contains(out, `"host": "db.local"`) {
		t.Errorf("outer field should appear, got:\n%s", out)
	}
	if !strings.Contains(out, `"password": "***"`) {
		t.Errorf("nested secret should be masked, got:\n%s", out)
	}
	if !strings.Contains(out, `"username": "dbuser"`) {
		t.Errorf("nested non-secret should appear, got:\n%s", out)
	}
}

func TestRedact_NilSafe(t *testing.T) {
	t.Parallel()

	out := Redact(nil)
	if out != "null" {
		t.Errorf("nil input should produce %q, got %q", "null", out)
	}

	// Nil pointer to struct
	var p *credentials
	out2 := Redact(p)
	if out2 != "null" {
		t.Errorf("nil pointer should produce %q, got %q", "null", out2)
	}
}

func TestRedact_SliceOfStructWalked(t *testing.T) {
	t.Parallel()

	ws := withSlice{
		Items: []credentials{
			{Username: "u1", Password: "p1"},
			{Username: "u2", Password: "p2"},
		},
	}
	out := Redact(ws)
	if strings.Contains(out, "p1") || strings.Contains(out, "p2") {
		t.Errorf("passwords inside slice should be masked, got:\n%s", out)
	}
	if !strings.Contains(out, "u1") || !strings.Contains(out, "u2") {
		t.Errorf("usernames inside slice should be visible, got:\n%s", out)
	}
}

func TestRedact_NilSlice(t *testing.T) {
	t.Parallel()

	ws := withSlice{Items: nil}
	out := Redact(ws)
	if !strings.Contains(out, `"items": null`) {
		t.Errorf("nil slice should render as null, got:\n%s", out)
	}
}
