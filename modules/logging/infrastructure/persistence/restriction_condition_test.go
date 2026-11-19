package persistence_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/logging/infrastructure/persistence"
	"github.com/stretchr/testify/require"
)

func TestRewriteCondition_LeavesLiteralsAndCommentsAlone(t *testing.T) {
	t.Parallel()
	args := []string{"$7", "$8"}
	tests := []struct{ name, in, out string }{
		{"placeholders and id", `{id} = ANY($1) AND {id} <> $2`, `ID = ANY($7) AND ID <> $8`},
		{"quoted literal", `{id} <> '$1 {id}'`, `ID <> '$1 {id}'`},
		{"doubled quote", `{id} <> 'it''s $1' AND $2 = {id}`, `ID <> 'it''s $1' AND $8 = ID`},
		{"escape string", `{id} <> E'\'$1' AND {id} = $1`, `ID <> E'\'$1' AND ID = $7`},
		{"quoted identifier", `"col$1" = {id}`, `"col$1" = ID`},
		{"identifier with dollar", `t.col$1 = $1`, `t.col$1 = $7`},
		{"line comment", "{id} = $1 -- $2 {id}\n", "ID = $7 -- $2 {id}\n"},
		{"block comment", `{id} = $1 /* $2 */`, `ID = $7 /* $2 */`},
		{"undeclared placeholder fails", `{id} = $3`, `ID = $0`},
		{"dollar quote fails", `{id} = $$x$$`, `ID = $0$x$$`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.out, persistence.RewriteCondition(tt.in, args, "ID"))
		})
	}
	// Without arguments a quoted '$1' is still compared as written, not rewritten to a valid '$0'.
	require.Equal(t, `ID <> '$1'`, persistence.RewriteCondition(`{id} <> '$1'`, nil, "ID"))
}
