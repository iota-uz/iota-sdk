package serrorlog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2/serrorlog"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestLogRequestBoundaryBoundedAttributes(t *testing.T) {
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	logger.SetFormatter(&logrus.JSONFormatter{})
	ctx := context.WithValue(context.Background(), constants.LoggerKey, logger.WithFields(logrus.Fields{"request-id": "synthetic-correlation", "private_payload": "private inherited diagnostic"}))
	err := serrors.NewInvalid("").WithOp("synthetic.op").WithReason("synthetic_reason").WithCause(errors.New("private SQL cause"))
	serrorlog.Log(ctx, err, "request failed")
	var fields map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &fields))
	require.Equal(t, "invalid", fields["error.code"])
	require.Equal(t, "synthetic.op", fields["error.op"])
	require.Equal(t, "synthetic_reason", fields["error.reason"])
	require.Equal(t, "synthetic-correlation", fields["request_id"])
	require.Equal(t, "info", fields["level"])
	require.NotContains(t, output.String(), "private")
	output.Reset()
	serrorlog.Log(context.Background(), err, "request failed")
	require.Empty(t, output.String())
}
