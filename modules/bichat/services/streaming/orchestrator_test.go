package streaming

import (
	"errors"
	"testing"

	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerminalChunk_Scenarios(t *testing.T) {
	testCases := []struct {
		name         string
		err          error
		generationMs int64
		wantType     bichatservices.ChunkType
		wantGenMs    int64
	}{
		{name: "done", err: nil, generationMs: 123, wantType: bichatservices.ChunkTypeDone, wantGenMs: 123},
		{name: "error", err: errors.New("boom"), generationMs: 0, wantType: bichatservices.ChunkTypeError, wantGenMs: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunk := TerminalChunk(tc.err, tc.generationMs)
			require.Equal(t, tc.wantType, chunk.Type)
			assert.Equal(t, tc.wantGenMs, chunk.GenerationMs)
			if tc.err == nil {
				require.NoError(t, chunk.Error)
			} else {
				require.Equal(t, tc.err, chunk.Error)
			}
		})
	}
}
