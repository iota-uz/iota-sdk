package uploadsconfig

import (
	"fmt"
	"strings"
	"time"
)

const (
	BackendFS = "fs"
	BackendS3 = "s3"
)

// StorageConfig selects the backend for new uploads and configures the
// optional S3-compatible object store. Reads are routed per stored path, so S3
// objects stay readable whenever S3 is configured, even when the primary
// backend is the filesystem.
//
// Env prefix: "upload" (e.g. UPLOAD_STORAGE_BACKEND → upload.storage.backend,
// UPLOAD_S3_ENDPOINT → upload.s3.endpoint, UPLOAD_S3_ACCESS_KEY →
// upload.s3.access.key, UPLOAD_S3_FORCE_PATH_STYLE → upload.s3.force.path.style,
// UPLOAD_S3_PRESIGN_TTL_SECONDS → upload.s3.presign.ttl.seconds).
type StorageConfig struct {
	Storage StorageBackendConfig `koanf:"storage"`
	S3      S3Config             `koanf:"s3"`
}

// StorageBackendConfig holds the primary backend name ("fs" or "s3").
type StorageBackendConfig struct {
	Backend string `koanf:"backend" default:"fs"`
}

// S3Config holds S3-compatible (MinIO, AWS) connection settings.
//
// Stored paths embed the bucket name (s3://<bucket>/<key>) and only paths of
// the configured bucket are readable. Changing UPLOAD_S3_BUCKET makes every
// existing s3:// upload unreadable; restoring the original value restores access.
type S3Config struct {
	Endpoint string           `koanf:"endpoint"`
	Bucket   string           `koanf:"bucket"`
	Region   string           `koanf:"region" default:"us-east-1"`
	Access   S3CredentialPart `koanf:"access"`
	Secret   S3CredentialPart `koanf:"secret"`
	Secure   bool             `koanf:"secure"`
	Force    S3ForceConfig    `koanf:"force"`
	Presign  S3PresignConfig  `koanf:"presign"`
}

// S3CredentialPart maps the trailing "_KEY" of UPLOAD_S3_ACCESS_KEY and UPLOAD_S3_SECRET_KEY.
type S3CredentialPart struct {
	Key string `koanf:"key" secret:"true"`
}

// S3ForceConfig maps UPLOAD_S3_FORCE_PATH_STYLE.
type S3ForceConfig struct {
	Path S3PathConfig `koanf:"path"`
}

// S3PathConfig maps UPLOAD_S3_FORCE_PATH_STYLE.
type S3PathConfig struct {
	Style bool `koanf:"style" default:"true"`
}

// S3PresignConfig maps UPLOAD_S3_PRESIGN_TTL_SECONDS.
type S3PresignConfig struct {
	TTL S3PresignTTLConfig `koanf:"ttl"`
}

// S3PresignTTLConfig maps UPLOAD_S3_PRESIGN_TTL_SECONDS.
type S3PresignTTLConfig struct {
	Seconds int `koanf:"seconds" default:"300"`
}

// ConfigPrefix returns the koanf prefix for StorageConfig ("upload").
func (StorageConfig) ConfigPrefix() string { return "upload" }

// Backend returns the normalized primary backend; empty means BackendFS.
func (c StorageConfig) Backend() string {
	backend := strings.ToLower(strings.TrimSpace(c.Storage.Backend))
	if backend == "" {
		return BackendFS
	}
	return backend
}

// S3Configured reports whether S3 reads and writes are possible.
func (c StorageConfig) S3Configured() bool {
	return c.S3.IsConfigured()
}

// Validate rejects unknown backends and an S3 primary without S3 settings.
func (c *StorageConfig) Validate() error {
	switch c.Backend() {
	case BackendFS:
		// S3 stays optional with the filesystem primary, but half of it is a typo.
		if c.S3.anySet() && !c.S3.IsConfigured() {
			return fmt.Errorf("partial S3 upload configuration: missing %s", c.S3.MissingFields())
		}
	case BackendS3:
		if !c.S3.IsConfigured() {
			return fmt.Errorf("UPLOAD_STORAGE_BACKEND=s3 requires %s", c.S3.MissingFields())
		}
	default:
		return fmt.Errorf("unsupported UPLOAD_STORAGE_BACKEND %q: want %q or %q", c.Storage.Backend, BackendFS, BackendS3)
	}
	if c.S3.Presign.TTL.Seconds < 0 {
		return fmt.Errorf("UPLOAD_S3_PRESIGN_TTL_SECONDS must not be negative")
	}
	return nil
}

// IsConfigured reports whether bucket and credentials are set.
func (c S3Config) IsConfigured() bool {
	return c.MissingFields() == ""
}

// MissingFields lists the unset required S3 env vars, or "" when none are missing.
func (c S3Config) MissingFields() string {
	var missing []string
	if strings.TrimSpace(c.Bucket) == "" {
		missing = append(missing, "UPLOAD_S3_BUCKET")
	}
	if strings.TrimSpace(c.Access.Key) == "" {
		missing = append(missing, "UPLOAD_S3_ACCESS_KEY")
	}
	if strings.TrimSpace(c.Secret.Key) == "" {
		missing = append(missing, "UPLOAD_S3_SECRET_KEY")
	}
	return strings.Join(missing, ", ")
}

func (c S3Config) anySet() bool {
	return strings.TrimSpace(c.Bucket) != "" || strings.TrimSpace(c.Access.Key) != "" || strings.TrimSpace(c.Secret.Key) != ""
}

// ForcePathStyle reports whether bucket names go into the URL path (MinIO) instead of the host.
func (c S3Config) ForcePathStyle() bool { return c.Force.Path.Style }

// PresignTTL returns the lifetime of presigned GET URLs.
func (c S3Config) PresignTTL() time.Duration {
	if c.Presign.TTL.Seconds <= 0 {
		return 300 * time.Second
	}
	return time.Duration(c.Presign.TTL.Seconds) * time.Second
}
