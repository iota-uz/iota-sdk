# Delegate Makefile for bichat/presentation/web
# This allows git hooks to run from this subdirectory

ROOT_DIR := ../../../..

fix:
	@cd $(ROOT_DIR) && $(MAKE) fix $(filter-out $@,$(MAKECMDGOALS))

vet:
	@cd $(ROOT_DIR) && go vet ./...

fmt imports:
	@:

.PHONY: fix fmt imports vet
