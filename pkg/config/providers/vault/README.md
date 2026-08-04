# Vault Provider (placeholder)

This package is a stub. The `Load` method is a no-op in Wave 0.1.

## Planned implementation

- Use `github.com/hashicorp/vault/api` (do **not** add to go.mod until this wave lands).
- `New(addr, token, path string)` connects to the Vault instance at `addr` using `token`.
- `Load` reads the KV secret at `path` and merges its key-value pairs into the koanf instance.
- Key transform should follow the same single-underscore-to-dot rule as the env provider.
- Missing secret path should be a configurable no-op or hard error (TBD in spec).

## References

- HashiCorp Vault Go client: https://github.com/hashicorp/vault/tree/main/api
- iota-sdk config spec: https://github.com/iota-uz/iota-sdk/issues/500
