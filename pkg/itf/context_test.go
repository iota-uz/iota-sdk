package itf

import (
	"errors"
	"testing"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/stretchr/testify/require"
)

type testController struct {
	key   string
	close func() error
}

func (c testController) Register(_ *mux.Router) {}

func (c testController) Key() string { return c.key }

func (c testController) Close() error {
	if c.close == nil {
		return nil
	}
	return c.close()
}

type plainController struct {
	key string
}

func (c plainController) Register(_ *mux.Router) {}

func (c plainController) Key() string { return c.key }

func TestCloseControllerResources_ClosesOnlyClosableControllers(t *testing.T) {
	t.Parallel()

	closedCount := 0
	controllers := []application.Controller{
		testController{
			key: "closable",
			close: func() error {
				closedCount++
				return nil
			},
		},
		plainController{key: "plain"},
	}

	closeControllerResources(t, controllers)
	require.Equal(t, 1, closedCount)
}

func TestCloseControllerResources_LogsCloseErrors(t *testing.T) {
	t.Parallel()

	controllers := []application.Controller{
		testController{
			key: "failing-closer",
			close: func() error {
				return errors.New("close failed")
			},
		},
	}

	closeControllerResources(t, controllers)
}
