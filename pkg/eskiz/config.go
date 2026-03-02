// Package eskiz provides this package.
package eskiz

type ConfigOption func(s *config)

type Config interface {
	URL() string
	Email() string
	Password() string
	CallbackURL() string
	MaxMessageSize() int
}

// WithCallbackURL sets the callback URL for SMS notifications
func WithCallbackURL(url string) ConfigOption {
	return func(c *config) {
		c.callbackURL = url
	}
}

// WithMaxMessageSize sets the maximum message size limit
func WithMaxMessageSize(size int) ConfigOption {
	return func(c *config) {
		c.maxMessageSize = size
	}
}

func NewConfig(
	url string,
	email string,
	password string,
	opts ...ConfigOption,
) Config {
	cfg := &config{
		url:            url,
		email:          email,
		password:       password,
		maxMessageSize: 500,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

type config struct {
	url            string
	email          string
	password       string
	callbackURL    string
	maxMessageSize int
}

func (c *config) URL() string {
	return c.url
}

func (c *config) Email() string {
	return c.email
}

func (c *config) Password() string {
	return c.password
}

func (c *config) CallbackURL() string {
	return c.callbackURL
}

func (c *config) MaxMessageSize() int {
	return c.maxMessageSize
}
