package crud

import (
	"github.com/gorilla/mux"
	formui "github.com/iota-uz/iota-sdk/components/scaffold/form"
)

// WithFields sets the form fields for the Schema
func WithFields[T any, ID any](fields ...formui.Field) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Service.Fields = fields
	}
}

// WithModelValidators adds model-level validators
func WithModelValidators[T any, ID any](vs ...ModelLevelValidator[T]) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Service.ModelValidators = vs
	}
}

// WithMiddlewares adds middleware functions to the Schema
func WithMiddlewares[T any, ID any](ms ...mux.MiddlewareFunc) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.middlewares = append(s.middlewares, ms...)
	}
}

// WithRenderer sets a custom renderer function
func WithRenderer[T any, ID any](renderer RenderFunc) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Renderer = renderer
	}
}

// WithGetPrimaryKey sets a custom function to get the primary key field name
func WithGetPrimaryKey[T any, ID any](fn func() string) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.getPrimaryKey = fn
	}
}

// WithEntityFactory sets a custom entity factory
func WithEntityFactory[T any, ID any](factory EntityFactory[T]) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Service.EntityFactory = factory
	}
}

// WithEntityPatcher sets a custom entity patcher
func WithEntityPatcher[T any, ID any](patcher EntityPatcher[T]) SchemaOpt[T, ID] {
	return func(s *Schema[T, ID]) {
		s.Service.EntityPatcher = patcher
	}
}
