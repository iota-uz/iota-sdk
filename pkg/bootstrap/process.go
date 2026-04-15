package bootstrap

import (
	"context"
	"log"
	"os"
	"runtime/debug"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func Main(run func() error) {
	defer func() {
		if r := recover(); r != nil {
			configuration.Use().Unload()
			log.Println(r)
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	if err := run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func NewIotaRuntime(
	ctx context.Context,
	conf *configuration.Configuration,
	serviceSuffix string,
) (*Runtime, func() error, error) {
	serviceName := conf.OpenTelemetry.ServiceName
	if serviceSuffix != "" && serviceName != "" {
		serviceName += "-" + serviceSuffix
	}
	return NewRuntime(ctx, IotaConfigWithServiceName(conf, serviceName))
}
