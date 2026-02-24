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

func TestCloseControllerResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		build          func() ([]application.Controller, *int)
		expectedClosed int
	}{
		{
			name: "closes only closable controllers",
			build: func() ([]application.Controller, *int) {
				closed := 0
				return []application.Controller{
					testController{
						key: "closable",
						close: func() error {
							closed++
							return nil
						},
					},
					plainController{key: "plain"},
				}, &closed
			},
			expectedClosed: 1,
		},
		{
			name: "logs close errors",
			build: func() ([]application.Controller, *int) {
				closed := 0
				return []application.Controller{
					testController{
						key: "failing-closer",
						close: func() error {
							closed++
							return errors.New("close failed")
						},
					},
				}, &closed
			},
			expectedClosed: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			controllers, closed := tt.build()
			closeControllerResources(t, controllers)
			require.Equal(t, tt.expectedClosed, *closed)
		})
	}
}
