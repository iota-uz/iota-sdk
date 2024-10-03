package controllers

import "github.com/go-playground/validator/v10"

var (
	validate = validator.New(validator.WithRequiredStructEnabled())
)
