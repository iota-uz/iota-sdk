package composables_test

import (
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMalformedRequestInputIsInvalidWithoutInventedFieldViolations(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		query      bool
	}{{"form syntax", "Amount=%secret", false}, {"form type", "Amount=private-diagnostic", false}, {"query type", "Amount=private-diagnostic", true}} {
		t.Run(tc.name, func(t *testing.T) {
			input := &struct{ Amount int }{}
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			var err error
			if tc.query {
				r = httptest.NewRequest(http.MethodGet, "/?"+tc.body, nil)
				_, err = composables.UseQuery(input, r)
			} else {
				_, err = composables.UseForm(input, r)
			}
			require.Error(t, err)
			require.True(t, serrors.HasCode(err, serrors.Invalid))
			require.Empty(t, serrors.FieldsOf(err))
			require.NotContains(t, serrors.Public(err, nil).Message, "private")
			require.NotContains(t, serrors.Public(err, nil).Message, "secret")
		})
	}
}
