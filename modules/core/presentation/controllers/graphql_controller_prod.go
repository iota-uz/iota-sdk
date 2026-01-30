//go:build !dev

package controllers

import "github.com/gorilla/mux"

func init() {
	registerPlaygroundHandler = func(router *mux.Router) {
		// No playground in production
	}
}
