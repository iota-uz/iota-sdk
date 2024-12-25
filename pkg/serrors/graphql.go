package serrors

import (
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func UnauthorizedGQLError(path ast.Path) *gqlerror.Error {
	return &gqlerror.Error{
		Path:    path,
		Message: "unauthorized",
		Extensions: map[string]interface{}{
			"code": "UNAUTHORIZED",
		},
	}
}
