package composition

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestAutoProvidersInjectFromBuildContext(t *testing.T) {
	logger := logrus.New()
	cfg := &configuration.Configuration{}

	engine := NewEngine()
	var seenLogger *logrus.Logger
	var seenConfig *configuration.Configuration

	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "consumer"},
		build: func(builder *Builder) error {
			Provide[string](builder, func(c *Container) (string, error) {
				l, err := Resolve[*logrus.Logger](c)
				if err != nil {
					return "", err
				}
				seenLogger = l
				cf, err := Resolve[*configuration.Configuration](c)
				if err != nil {
					return "", err
				}
				seenConfig = cf
				return "ok", nil
			})
			return nil
		},
	})
	require.NoError(t, err)

	bctx := BuildContext{
		logger: logger,
		config: cfg,
	}

	container, err := engine.Compile(bctx)
	require.NoError(t, err)

	value, err := Resolve[string](container)
	require.NoError(t, err)
	require.Equal(t, "ok", value)
	require.Same(t, logger, seenLogger)
	require.Same(t, cfg, seenConfig)
}

func TestAutoProvidersOverridableByUserProvider(t *testing.T) {
	original := logrus.New()
	override := logrus.New()

	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "logger-override"},
		build: func(builder *Builder) error {
			Provide[*logrus.Logger](builder, override)
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{logger: original})
	require.NoError(t, err)

	resolved, err := Resolve[*logrus.Logger](container)
	require.NoError(t, err)
	require.Same(t, override, resolved)
}
