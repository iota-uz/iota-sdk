package expr

import (
	"strings"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

// joinFunction declares `join(list<string>, sep)`, the global form of the member `list.join(sep)`: a
// template reads `join(issue.labels, "\n• ")` where a chain of selects would break the line.
func joinFunction() cel.EnvOption {
	return cel.Function("join",
		cel.Overload("join_list_string_global", []*cel.Type{cel.ListType(cel.StringType), cel.StringType}, cel.StringType,
			cel.BinaryBinding(func(list, sep ref.Val) ref.Val {
				l, ok := list.(traits.Lister)
				if !ok {
					return types.MaybeNoSuchOverloadErr(list)
				}
				s, ok := sep.(types.String)
				if !ok {
					return types.MaybeNoSuchOverloadErr(sep)
				}
				n, ok := l.Size().(types.Int)
				if !ok {
					return types.MaybeNoSuchOverloadErr(list)
				}
				parts := make([]string, 0, int(n))
				for i := 0; i < int(n); i++ {
					e, ok := l.Get(types.Int(i)).(types.String)
					if !ok {
						return types.NewErr("join: element %d is not a string", i)
					}
					parts = append(parts, string(e))
				}
				return types.String(strings.Join(parts, string(s)))
			}),
		),
	)
}
