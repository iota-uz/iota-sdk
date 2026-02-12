package tools

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifySQLError_UndefinedColumn(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{
		Code:    "42703",
		Message: `column "user_name" does not exist`,
	}

	diagnosis := ClassifySQLError(pgErr)

	if diagnosis.Code != ErrCodeColumnNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeColumnNotFound, diagnosis.Code)
	}
	if diagnosis.Column != "user_name" {
		t.Errorf("expected column 'user_name', got '%s'", diagnosis.Column)
	}
	if !strings.Contains(diagnosis.Message, "user_name") {
		t.Errorf("expected message to contain 'user_name', got: %s", diagnosis.Message)
	}
	if !strings.Contains(diagnosis.Suggestion, "user_name") {
		t.Errorf("expected suggestion to mention column name, got: %s", diagnosis.Suggestion)
	}
}

func TestClassifySQLError_UndefinedTable(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{
		Code:    "42P01",
		Message: `relation "users" does not exist`,
	}

	diagnosis := ClassifySQLError(pgErr)

	if diagnosis.Code != ErrCodeTableNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeTableNotFound, diagnosis.Code)
	}
	if diagnosis.Table != "users" {
		t.Errorf("expected table 'users', got '%s'", diagnosis.Table)
	}
	if !strings.Contains(diagnosis.Message, "users") {
		t.Errorf("expected message to contain 'users', got: %s", diagnosis.Message)
	}
	if !strings.Contains(diagnosis.Suggestion, "schema_list") {
		t.Errorf("expected suggestion to mention schema_list, got: %s", diagnosis.Suggestion)
	}
}

func TestClassifySQLError_TypeMismatch(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{
		Code:    "42804",
		Message: "column \"price\" is of type numeric but expression is of type text",
	}

	diagnosis := ClassifySQLError(pgErr)

	if diagnosis.Code != ErrCodeTypeMismatch {
		t.Errorf("expected code %s, got %s", ErrCodeTypeMismatch, diagnosis.Code)
	}
	if !strings.Contains(diagnosis.Message, "numeric") {
		t.Errorf("expected message to contain type info, got: %s", diagnosis.Message)
	}
	if !strings.Contains(diagnosis.Suggestion, "cast") {
		t.Errorf("expected suggestion to mention casting, got: %s", diagnosis.Suggestion)
	}
}

func TestClassifySQLError_SyntaxError(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{
		Code:    "42601",
		Message: `syntax error at or near "SELCT"`,
	}

	diagnosis := ClassifySQLError(pgErr)

	if diagnosis.Code != ErrCodeSyntaxError {
		t.Errorf("expected code %s, got %s", ErrCodeSyntaxError, diagnosis.Code)
	}
	if !strings.Contains(diagnosis.Message, "syntax error") {
		t.Errorf("expected message to contain 'syntax error', got: %s", diagnosis.Message)
	}
	if !strings.Contains(diagnosis.Suggestion, "syntax") {
		t.Errorf("expected suggestion to mention syntax, got: %s", diagnosis.Suggestion)
	}
}

func TestClassifySQLError_AmbiguousColumn(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{
		Code:    "42702",
		Message: `column "id" is ambiguous`,
	}

	diagnosis := ClassifySQLError(pgErr)

	if diagnosis.Code != ErrCodeAmbiguousColumn {
		t.Errorf("expected code %s, got %s", ErrCodeAmbiguousColumn, diagnosis.Code)
	}
	if diagnosis.Column != "id" {
		t.Errorf("expected column 'id', got '%s'", diagnosis.Column)
	}
	if !strings.Contains(diagnosis.Suggestion, "alias") {
		t.Errorf("expected suggestion to mention alias, got: %s", diagnosis.Suggestion)
	}
	if !strings.Contains(diagnosis.Suggestion, "id") {
		t.Errorf("expected suggestion to mention column name, got: %s", diagnosis.Suggestion)
	}
}

func TestClassifySQLError_NilError(t *testing.T) {
	t.Parallel()

	diagnosis := ClassifySQLError(nil)

	if diagnosis.Code != ErrCodeQueryError {
		t.Errorf("expected code %s, got %s", ErrCodeQueryError, diagnosis.Code)
	}
	if diagnosis.Message != "unknown error" {
		t.Errorf("expected 'unknown error' message, got: %s", diagnosis.Message)
	}
	if len(diagnosis.Hints) == 0 {
		t.Error("expected hints to be present")
	}
}

func TestClassifySQLError_NonPgError(t *testing.T) {
	t.Parallel()

	err := errors.New("connection refused")

	diagnosis := ClassifySQLError(err)

	if diagnosis.Code != ErrCodeQueryError {
		t.Errorf("expected code %s, got %s", ErrCodeQueryError, diagnosis.Code)
	}
	if !strings.Contains(diagnosis.Message, "connection refused") {
		t.Errorf("expected message to contain 'connection refused', got: %s", diagnosis.Message)
	}
	if len(diagnosis.Hints) == 0 {
		t.Error("expected hints to be present")
	}
}

func TestClassifySQLError_UnknownPgCode(t *testing.T) {
	t.Parallel()

	pgErr := &pgconn.PgError{
		Code:    "99999",
		Message: "some unknown error",
	}

	diagnosis := ClassifySQLError(pgErr)

	if diagnosis.Code != ErrCodeQueryError {
		t.Errorf("expected code %s, got %s", ErrCodeQueryError, diagnosis.Code)
	}
	if !strings.Contains(diagnosis.Message, "some unknown error") {
		t.Errorf("expected original message, got: %s", diagnosis.Message)
	}
	if !strings.Contains(diagnosis.Suggestion, "99999") {
		t.Errorf("expected suggestion to contain error code, got: %s", diagnosis.Suggestion)
	}
}
