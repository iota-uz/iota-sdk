package testharness

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	DefaultServerURL           = "http://127.0.0.1:3200"
	DefaultGraphQLEndpointPath = "/query/ali"
	DefaultStreamEndpointPath  = "/admin/ali/chat/stream"
	DefaultCookieName          = "granite_sid"
	DefaultJudgeModel          = "gpt-5-mini"
	DefaultParallelWorkers     = 8
	DefaultGraphQLPollTimeout  = 15
	DefaultGraphQLPollInterval = 250
	DefaultStreamDoneDrainMS   = 3000
)

type Config struct {
	ServerURL           string
	GraphQLEndpointPath string
	StreamEndpointPath  string
	CookieName          string
	SessionToken        string

	JudgeModel   string
	OpenAIAPIKey string
	DisableJudge bool

	Parallel     int
	CacheEnabled bool
	CacheDir     string

	ArtifactsDir string

	GraphQLPollTimeoutSeconds int
	GraphQLPollIntervalMillis int
	StreamDoneDrainMillis     int

	IotaSDKRevision string
	HostRevision    string
}

func (c *Config) ApplyDefaults() {
	if c.ServerURL == "" {
		c.ServerURL = DefaultServerURL
	}
	if c.GraphQLEndpointPath == "" {
		c.GraphQLEndpointPath = DefaultGraphQLEndpointPath
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
	if c.GraphQLPollTimeoutSeconds <= 0 {
		c.GraphQLPollTimeoutSeconds = DefaultGraphQLPollTimeout
	}
	if c.GraphQLPollIntervalMillis <= 0 {
		c.GraphQLPollIntervalMillis = DefaultGraphQLPollInterval
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
	if !strings.HasPrefix(c.GraphQLEndpointPath, "/") {
		return errors.New("graphql_endpoint_path must start with /")
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
