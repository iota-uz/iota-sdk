//go:build !dev

// Package controllers provides this package.
package controllers

// initDevPlayground is a no-op in production builds.
func initDevPlayground(_ *GraphQLController) {}
