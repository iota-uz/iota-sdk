package applet

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type Procedure[P any, R any] struct {
	RequirePermissions []string
	Handler            func(ctx context.Context, params P) (R, error)
}

type TypedRPCRouter struct {
	procs []*typedProcedure
}

type typedProcedure struct {
	name               string
	requirePermissions []string
	paramType          reflect.Type
	resultType         reflect.Type
	method             RPCMethod
}

func NewTypedRPCRouter() *TypedRPCRouter {
	return &TypedRPCRouter{procs: make([]*typedProcedure, 0)}
}

func AddProcedure[P any, R any](r *TypedRPCRouter, name string, p Procedure[P, R]) {
	const op serrors.Op = "TypedRPCRouter.Add"

	if r == nil {
		panic("TypedRPCRouter is nil")
	}

	paramType := reflect.TypeOf((*P)(nil)).Elem()
	resultType := reflect.TypeOf((*R)(nil)).Elem()

	method := RPCMethod{
		RequirePermissions: p.RequirePermissions,
		Handler: func(ctx context.Context, params json.RawMessage) (any, error) {
			var decoded P
			if len(params) > 0 && string(params) != "null" {
				if err := json.Unmarshal(params, &decoded); err != nil {
					return nil, serrors.E(op, serrors.Invalid, "invalid params", err)
				}
			}
			res, err := p.Handler(ctx, decoded)
			if err != nil {
				return nil, err
			}
			return res, nil
		},
	}

	r.procs = append(r.procs, &typedProcedure{
		name:               name,
		requirePermissions: p.RequirePermissions,
		paramType:          paramType,
		resultType:         resultType,
		method:             method,
	})
}

func (r *TypedRPCRouter) Config() *RPCConfig {
	methods := make(map[string]RPCMethod, len(r.procs))
	for _, p := range r.procs {
		methods[p.name] = p.method
	}

	return &RPCConfig{
		Path:    "/rpc",
		Methods: methods,
	}
}
