package subscription

import "time"

type Config struct {
	Stripe                StripeConfig
	Cache                 CacheConfig
	GracePeriodDays       int
	LimitWarningThreshold float64
	DefaultTier           string
	Features              []FeatureDefinition
	Tiers                 []TierDefinition
}

type StripeConfig struct {
	SecretKey      string
	WebhookSecret  string
	PublishableKey string
}

type CacheConfig struct {
	Type       string
	TTLMinutes int
	RedisURL   string
}

type FeatureDefinition struct {
	Key         string
	Name        string
	Description string
	Type        string
}

func (c Config) normalized() Config {
	out := c
	if out.GracePeriodDays <= 0 {
		out.GracePeriodDays = 7
	}
	if out.LimitWarningThreshold <= 0 || out.LimitWarningThreshold > 1 {
		out.LimitWarningThreshold = 0.8
	}
	if out.DefaultTier == "" {
		out.DefaultTier = "FREE"
	}
	if out.Cache.Type == "" {
		out.Cache.Type = "memory"
	}
	if out.Cache.TTLMinutes <= 0 {
		out.Cache.TTLMinutes = 5
	}
	return out
}

func (c Config) cacheTTL() time.Duration {
	return time.Duration(c.Cache.TTLMinutes) * time.Minute
}
