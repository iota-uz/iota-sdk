// Package serrors is the unstable preview of the canonical error API.
package serrors

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Code uint8

const (
	Internal Code = iota + 1
	Invalid
	NotFound
	AlreadyExists
	Conflict
	FailedPrecondition
	PermissionDenied
	Unauthenticated
	RateLimited
	Unavailable
	Timeout
	Canceled
	Unimplemented
)

func (c Code) String() string {
	switch c {
	case Internal:
		return "internal"
	case Invalid:
		return "invalid"
	case NotFound:
		return "not_found"
	case AlreadyExists:
		return "already_exists"
	case Conflict:
		return "conflict"
	case FailedPrecondition:
		return "failed_precondition"
	case PermissionDenied:
		return "permission_denied"
	case Unauthenticated:
		return "unauthenticated"
	case RateLimited:
		return "rate_limited"
	case Unavailable:
		return "unavailable"
	case Timeout:
		return "timeout"
	case Canceled:
		return "canceled"
	case Unimplemented:
		return "unimplemented"
	default:
		return "internal"
	}
}

type Op string
type Reason string

// Value limits localization arguments to text, numbers, booleans and messages.
// Its private representation prevents arbitrary objects entering projections.
type Value struct {
	scalar  any
	message *Message
}

func Text(v string) Value       { return Value{scalar: v} }
func Number(v int64) Value      { return Value{scalar: v} }
func Boolean(v bool) Value      { return Value{scalar: v} }
func Reference(v Message) Value { m := cloneMessage(v); return Value{message: &m} }

type Message struct {
	ID    string
	Text  string
	Args  map[string]Value
	Count *int64
}

type FieldViolation struct {
	Field   string
	Reason  Reason
	Message Message
}

// Error holds internal diagnostics. Public data is added explicitly by builders.
// Private fields keep shared sentinel instances immutable.
type Error struct {
	code   Code
	op     Op
	msg    string
	err    error
	reason Reason
	public Message
	fields []FieldViolation
	meta   map[string]Value
}

func New(code Code, msg string) *Error {
	if code < Internal || code > Unimplemented {
		code = Internal
	}
	return &Error{code: code, msg: msg}
}

func Wrap(op Op, err error) error {
	if err == nil {
		return nil
	}
	return &Error{op: op, err: err}
}

// WrapContext adds private diagnostic context without introducing a new code.
func WrapContext(op Op, err error, diagnostic string) error {
	if err == nil {
		return nil
	}
	return &Error{op: op, err: err, msg: diagnostic}
}

func FromDB(op Op, err error) error {
	if err == nil {
		return nil
	}
	if code := CodeOf(err); frame(err) != nil || code == Canceled || code == Timeout {
		return Wrap(op, err)
	}
	code := Internal
	switch {
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, pgx.ErrNoRows):
		code = NotFound
	case errors.Is(err, context.Canceled):
		code = Canceled
	case errors.Is(err, context.DeadlineExceeded):
		code = Timeout
	}
	return New(code, "").WithOp(op).WithCause(err)
}

// FromDBContext classifies a driver error and retains private diagnostic context.
func FromDBContext(op Op, err error, diagnostic string) error {
	return WrapContext(op, FromDB("", err), diagnostic)
}

// Constraint maps only repository-owned, explicitly recognized constraints.
type Constraint struct {
	SQLState string
	Name     string
	Table    string
	Column   string
	Code     Code
	Reason   Reason
	Message  Message
}

func FromConstraint(op Op, err error, rules ...Constraint) error {
	if err == nil {
		return nil
	}
	if code := CodeOf(err); frame(err) != nil || code == Canceled || code == Timeout {
		return Wrap(op, err)
	}
	var pg interface{ SQLState() string }
	var named interface{ ConstraintName() string }
	// PostgreSQL's concrete error exposes the name as a field, not a method.
	name := constraintName(err)
	if errors.As(err, &named) {
		name = named.ConstraintName()
	}
	if errors.As(err, &pg) {
		for _, rule := range rules {
			if rule.SQLState != pg.SQLState() {
				continue
			}
			if rule.SQLState == "23502" && rule.Name == "" {
				table, column := constraintColumn(err)
				if rule.Table == "" || rule.Column == "" || table != rule.Table || column != rule.Column {
					continue
				}
			} else if rule.Name == "" || rule.Name != name {
				continue
			}
			var code Code
			switch rule.SQLState {
			case "23505":
				code = AlreadyExists
			case "23503":
				code = Conflict
			case "23502", "23514":
				code = Invalid
			default:
				continue
			}
			if rule.Code != 0 && rule.Code != code {
				continue
			}
			return New(code, "").WithCause(err).WithOp(op).WithReason(rule.Reason).WithPublic(rule.Message)
		}
	}
	return FromDB(op, err)
}

func (e *Error) Error() string {
	parts := []string{}
	if e.op != "" {
		parts = append(parts, string(e.op))
	}
	if e.msg != "" {
		parts = append(parts, e.msg)
	}
	if e.err != nil {
		parts = append(parts, e.err.Error())
	}
	if len(parts) == 0 {
		return CodeOf(e).String()
	}
	return strings.Join(parts, ": ")
}

func (e *Error) Unwrap() error     { return e.err }
func (e *Error) Operation() string { return string(e.op) }
func (e *Error) ErrorKind() string {
	if len(FieldsOf(e)) > 0 {
		return "validation"
	}
	if CodeOf(e) == PermissionDenied {
		return "forbidden"
	}
	return CodeOf(e).String()
}

func (e *Error) copy() *Error {
	c := *e
	c.public = cloneMessage(e.public)
	c.fields = cloneFields(e.fields)
	c.meta = cloneValues(e.meta)
	return &c
}

func (e *Error) WithOp(op Op) *Error             { c := e.copy(); c.op = op; return c }
func (e *Error) WithCause(err error) *Error      { c := e.copy(); c.err = err; return c }
func (e *Error) WithReason(reason Reason) *Error { c := e.copy(); c.reason = reason; return c }
func (e *Error) WithPublic(message Message) *Error {
	c := e.copy()
	c.public = cloneMessage(message)
	return c
}
func (e *Error) WithFields(fields ...FieldViolation) *Error {
	c := e.copy()
	c.fields = cloneFields(fields)
	return c
}
func (e *Error) WithMeta(meta map[string]Value) *Error {
	c := e.copy()
	c.meta = cloneValues(meta)
	return c
}

func walk(err error, visit func(error) bool) bool {
	if err == nil {
		return false
	}
	if visit(err) {
		return true
	}
	switch e := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range e.Unwrap() {
			if walk(child, visit) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return walk(e.Unwrap(), visit)
	}
	return false
}

func CodeOf(err error) Code {
	code := Internal
	walk(err, func(err error) bool {
		if e, ok := err.(*Error); ok && e.code != 0 {
			code = e.code
			return true
		}
		if err == context.Canceled {
			code = Canceled
			return true
		}
		if err == context.DeadlineExceeded {
			code = Timeout
			return true
		}
		return false
	})
	return code
}

func HasCode(err error, code Code) bool { return err != nil && CodeOf(err) == code }

// frame selects the first explicit classification, so outer Internal cannot
// accidentally inherit public fields or messages from an inner client error.
func frame(err error) *Error {
	var result *Error
	walk(err, func(err error) bool {
		if e, ok := err.(*Error); ok && e.code != 0 {
			result = e
			return true
		}
		return err == context.Canceled || err == context.DeadlineExceeded
	})
	return result
}

func OpOf(err error) Op {
	var op Op
	walk(err, func(err error) bool {
		if e, ok := err.(*Error); ok && e.op != "" {
			op = e.op
			return true
		}
		return false
	})
	return op
}
func Trace(err error) []Op {
	var result []Op
	walk(err, func(err error) bool {
		if e, ok := err.(*Error); ok && e.op != "" {
			result = append(result, e.op)
		}
		return false
	})
	return result
}
func ReasonOf(err error) Reason {
	if e := frame(err); e != nil {
		return e.reason
	}
	return ""
}
func FieldsOf(err error) []FieldViolation {
	if e := frame(err); e != nil {
		return cloneFields(e.fields)
	}
	return nil
}
func MetaOf(err error) map[string]Value {
	if e := frame(err); e != nil {
		return cloneValues(e.meta)
	}
	return nil
}
func MessageOf(err error) Message {
	if e := frame(err); e != nil {
		return cloneMessage(e.public)
	}
	return Message{}
}
func Multi(errs ...error) error { return errors.Join(errs...) }

func cloneMessage(m Message) Message {
	m.Args = cloneValues(m.Args)
	if m.Count != nil {
		count := *m.Count
		m.Count = &count
	}
	return m
}
func cloneValues(values map[string]Value) map[string]Value {
	if values == nil {
		return nil
	}
	result := make(map[string]Value, len(values))
	for key, v := range values {
		if v.message != nil {
			m := cloneMessage(*v.message)
			v.message = &m
		}
		result[key] = v
	}
	return result
}
func cloneFields(fields []FieldViolation) []FieldViolation {
	if fields == nil {
		return nil
	}
	result := append([]FieldViolation(nil), fields...)
	for i := range result {
		result[i].Message = cloneMessage(result[i].Message)
	}
	return result
}
