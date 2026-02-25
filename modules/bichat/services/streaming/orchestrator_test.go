package streaming

import (
	"errors"
	"testing"

	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/stretchr/testify/require"
)

func TestTerminalChunk_Done(t *testing.T) {
	chunk := TerminalChunk(nil, 123)
	require.Equal(t, bichatservices.ChunkTypeDone, chunk.Type)
	require.Equal(t, int64(123), chunk.GenerationMs)
	require.NoError(t, chunk.Error)
}

func TestTerminalChunk_Error(t *testing.T) {
	err := errors.New("boom")
	chunk := TerminalChunk(err, 0)
	require.Equal(t, bichatservices.ChunkTypeError, chunk.Type)
	require.Equal(t, err, chunk.Error)
}
