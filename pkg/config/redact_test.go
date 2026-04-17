package config

import (
	"encoding/json"
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

// --- map support ------------------------------------------------------------

type withMap struct {
	Labels map[string]string `json:"labels"`
}

type withSecretMap struct {
	Tokens map[string]string `json:"tokens" secret:"true"`
}

func TestRedact_MapWithoutSecret_RecursesValues(t *testing.T) {
	t.Parallel()

	v := withMap{Labels: map[string]string{"env": "prod", "region": "us-east-1"}}
	out := Redact(v)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	labels, ok := parsed["labels"].(map[string]any)
	if !ok {
		t.Fatalf("labels field missing or wrong type: %s", out)
	}
	if labels["env"] != "prod" {
		t.Errorf("labels.env: got %v, want %q", labels["env"], "prod")
	}
	if labels["region"] != "us-east-1" {
		t.Errorf("labels.region: got %v, want %q", labels["region"], "us-east-1")
	}
}

func TestRedact_MapWithSecretTag_RedactsWhole(t *testing.T) {
	t.Parallel()

	v := withSecretMap{Tokens: map[string]string{"jwt": "tok-secret", "api": "key-secret"}}
	out := Redact(v)

	if strings.Contains(out, "tok-secret") || strings.Contains(out, "key-secret") {
		t.Errorf("secret map contents leaked: %s", out)
	}
	if !strings.Contains(out, `"tokens": "***"`) {
		t.Errorf("secret map should be redacted as '***': %s", out)
	}
}

func TestRedact_EmptySecretMap_EmitsEmptyString(t *testing.T) {
	t.Parallel()

	v := withSecretMap{Tokens: map[string]string{}}
	out := Redact(v)

	if !strings.Contains(out, `"tokens": ""`) {
		t.Errorf("empty secret map should emit empty string: %s", out)
	}
}

func TestRedact_NilSecretMap_EmitsEmptyString(t *testing.T) {
	t.Parallel()

	v := withSecretMap{Tokens: nil}
	out := Redact(v)

	if !strings.Contains(out, `"tokens": ""`) {
		t.Errorf("nil secret map should emit empty string: %s", out)
	}
}

// --- nested struct secret tags ----------------------------------------------

type innerSecret struct {
	Key string
	Val string
}

type withSecretStruct struct {
	Creds innerSecret `json:"creds" secret:"true"`
}

func TestRedact_NestedStructSecretTag_NoRecursion(t *testing.T) {
	t.Parallel()

	v := withSecretStruct{Creds: innerSecret{Key: "admin", Val: "hunter2"}}
	out := Redact(v)

	if strings.Contains(out, "admin") || strings.Contains(out, "hunter2") {
		t.Errorf("secret nested struct contents leaked: %s", out)
	}
	if !strings.Contains(out, `"creds": "***"`) {
		t.Errorf("secret nested struct should be '***': %s", out)
	}
}

func TestRedact_ZeroNestedStructSecretTag_EmitsEmptyString(t *testing.T) {
	t.Parallel()

	v := withSecretStruct{Creds: innerSecret{}}
	out := Redact(v)

	if !strings.Contains(out, `"creds": ""`) {
		t.Errorf("zero secret nested struct should emit empty string: %s", out)
	}
}

// --- slice secret tags ------------------------------------------------------

type withSecretSlice struct {
	Keys []string `json:"keys" secret:"true"`
}

func TestRedact_NonEmptySecretSlice_Redacts(t *testing.T) {
	t.Parallel()

	v := withSecretSlice{Keys: []string{"key1", "key2"}}
	out := Redact(v)

	if strings.Contains(out, "key1") || strings.Contains(out, "key2") {
		t.Errorf("secret slice contents leaked: %s", out)
	}
	if !strings.Contains(out, `"keys": "***"`) {
		t.Errorf("secret slice should be '***': %s", out)
	}
}

func TestRedact_EmptySecretSlice_EmitsEmptyString(t *testing.T) {
	t.Parallel()

	v := withSecretSlice{Keys: []string{}}
	out := Redact(v)

	if !strings.Contains(out, `"keys": ""`) {
		t.Errorf("empty secret slice should emit empty string: %s", out)
	}
}

func TestRedact_NilSecretSlice_EmitsEmptyString(t *testing.T) {
	t.Parallel()

	v := withSecretSlice{Keys: nil}
	out := Redact(v)

	if !strings.Contains(out, `"keys": ""`) {
		t.Errorf("nil secret slice should emit empty string: %s", out)
	}
}
