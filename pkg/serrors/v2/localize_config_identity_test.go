package serrors_test

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMessageFromConfigRejectsMismatchedIdentity(t *testing.T) {
	_, err := serrors.MessageFromConfig(&i18n.LocalizeConfig{MessageID: "requested", DefaultMessage: &i18n.Message{ID: "different", Other: "safe authored text"}})
	require.Error(t, err)
}
