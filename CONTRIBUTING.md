# Contributing

## CI Base Image Requirement

This repository's CI workflow uses an immutable container tag for the CI base image.

- Required repository variable: `CI_BASE_IMAGE_SHA`
- Expected image format: `ghcr.io/<owner>/iota-sdk-ci-base:sha-<CI_BASE_IMAGE_SHA>`
- If `CI_BASE_IMAGE_SHA` is not set, CI fails fast in the `validate-ci-base-image` job.

### How to update

1. Run the **Build CI Base Image** workflow (`.github/workflows/ci-base-image.yml`) on `main`.
2. Take the pushed image tag suffix from `sha-<value>`.
3. Update repository variable `CI_BASE_IMAGE_SHA` to `<value>`.
4. Re-run PR checks.

### Fork/bootstrap note

If you run CI in a fork, either:

1. Publish your own `iota-sdk-ci-base` image and set `CI_BASE_IMAGE_SHA`, or
2. Override workflow/container strategy in your fork.
