package testharness

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	DefaultServerURL          = "http://127.0.0.1:3200"
	DefaultRPCEndpointPath    = "/bi-chat/rpc"
	DefaultStreamEndpointPath = "/bi-chat/stream"
	DefaultCookieName         = "granite_sid"
	DefaultJudgeModel         = "gpt-5-mini"
	DefaultParallelWorkers    = 8
	DefaultRPCPollTimeout     = 15
	DefaultRPCPollInterval    = 250
	DefaultStreamDoneDrainMS  = 3000
)

type Config struct {
	ServerURL          string
	RPCEndpointPath    string
	StreamEndpointPath string
	CookieName         string
	SessionToken       string

	JudgeModel   string
	OpenAIAPIKey string
	DisableJudge bool

	Parallel     int
	FailFast     bool
	CacheEnabled bool
	CacheDir     string

	ArtifactsDir string
	OracleFacts  map[string]OracleFact

	RPCPollTimeoutSeconds int
	RPCPollIntervalMillis int
	StreamDoneDrainMillis int

	IotaSDKRevision string
	HostRevision    string
}

func (c *Config) ApplyDefaults() {
	if c.ServerURL == "" {
		c.ServerURL = DefaultServerURL
	}
	if c.RPCEndpointPath == "" {
		c.RPCEndpointPath = DefaultRPCEndpointPath
	}
	if c.StreamEndpointPath == "" {
		c.StreamEndpointPath = DefaultStreamEndpointPath
	}
	if c.CookieName == "" {
		c.CookieName = DefaultCookieName
	}
	if c.JudgeModel == "" {
		c.JudgeModel = DefaultJudgeModel
	}
	if c.RPCPollTimeoutSeconds <= 0 {
		c.RPCPollTimeoutSeconds = DefaultRPCPollTimeout
	}
	if c.RPCPollIntervalMillis <= 0 {
		c.RPCPollIntervalMillis = DefaultRPCPollInterval
	}
	if c.StreamDoneDrainMillis <= 0 {
		c.StreamDoneDrainMillis = DefaultStreamDoneDrainMS
	}
}

func (c *Config) Validate() error {
	c.ApplyDefaults()

	if c.CookieName == "" {
		return errors.New("cookie_name is required")
	}

	if _, err := url.ParseRequestURI(c.ServerURL); err != nil {
		return errors.New("server_url is invalid")
	}
	if !strings.HasPrefix(c.RPCEndpointPath, "/") {
		return errors.New("rpc_endpoint_path must start with /")
	}
	if !strings.HasPrefix(c.StreamEndpointPath, "/") {
		return errors.New("stream_endpoint_path must start with /")
	}
	if !c.DisableJudge && c.OpenAIAPIKey == "" {
		return errors.New("openai_api_key is required when judge is enabled")
	}
	if c.CacheEnabled {
		if c.CacheDir == "" {
			return errors.New("cache_dir is required when cache is enabled")
		}
		c.CacheDir = filepath.Clean(c.CacheDir)
	}
	return nil
}

func (c *Config) EffectiveParallelism() int {
	if c.Parallel > 0 {
		return c.Parallel
	}
	return DefaultParallelWorkers
}
