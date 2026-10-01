package controllers_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/jobs/presentation/controllers/dtos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func formRequest(t *testing.T, values url.Values) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/jobs", strings.NewReader(values.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestParseCreateJobDTO_KindAndJSONParams(t *testing.T) {
	req := formRequest(t, url.Values{
		"kind":   {"report.export"},
		"params": {`{"format":"xlsx","limit":100}`},
	})

	dto, err := dtos.ParseCreateJobDTO(req)

	require.NoError(t, err)
	assert.Equal(t, "report.export", dto.Kind)
	assert.Equal(t, "xlsx", dto.Params["format"])
	assert.Equal(t, float64(100), dto.Params["limit"])
}

func TestParseCreateJobDTO_PrefixedFormParams(t *testing.T) {
	req := formRequest(t, url.Values{
		"kind":           {"bulk.delete"},
		"params.scope":   {"all"},
		"params.dry_run": {"true"},
	})

	dto, err := dtos.ParseCreateJobDTO(req)

	require.NoError(t, err)
	assert.Equal(t, "all", dto.Params["scope"])
	assert.Equal(t, "true", dto.Params["dry_run"])
}

func TestParseCreateJobDTO_NoParams(t *testing.T) {
	req := formRequest(t, url.Values{"kind": {"noop"}})

	dto, err := dtos.ParseCreateJobDTO(req)

	require.NoError(t, err)
	assert.Equal(t, "noop", dto.Kind)
	assert.NotNil(t, dto.Params)
	assert.Empty(t, dto.Params)
}

func TestParseCreateJobDTO_InvalidJSONRejected(t *testing.T) {
	req := formRequest(t, url.Values{
		"kind":   {"export"},
		"params": {`{not-json`},
	})

	_, err := dtos.ParseCreateJobDTO(req)

	require.Error(t, err)
}
