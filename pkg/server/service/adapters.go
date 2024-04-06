package service

import "github.com/graphql-go/graphql"

type GraphQLAdapterOptions struct {
	Service Service
	Name    string
}

func GraphQLAdapter(opts *GraphQLAdapterOptions) (*graphql.Object, *graphql.Object) {
	pkCol := opts.Service.Model().Pk
	fields := graphql.Fields{
		pkCol: &graphql.Field{
			Type: graphql.Int,
		},
	}
	for _, field := range opts.Service.Model().Fields {
		fields[field.Name] = &graphql.Field{
			Type: graphql.String,
		}
	}
	var modelType = graphql.NewObject(
		graphql.ObjectConfig{
			Name:   opts.Name + "Type",
			Fields: fields,
		},
	)

	queryType := graphql.NewObject(
		graphql.ObjectConfig{
			Name: opts.Name,
			Fields: graphql.Fields{
				"get": &graphql.Field{
					Type:        modelType,
					Description: "Get by id",
					Args: graphql.FieldConfigArgument{
						pkCol: &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id, ok := p.Args[pkCol].(int)
						if ok {
							return opts.Service.Get(&GetQuery{Id: int64(id)})
						}
						return nil, nil
					},
				},
				"list": &graphql.Field{
					Type:        graphql.NewList(modelType),
					Description: "Get list",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return opts.Service.Find(&FindQuery{})
					},
				},
			},
		},
	)
	createArgs := graphql.FieldConfigArgument{}
	for _, field := range opts.Service.Model().Fields {
		createArgs[field.Name] = &graphql.ArgumentConfig{
			Type: graphql.String,
		}
	}
	mutationType := graphql.NewObject(
		graphql.ObjectConfig{
			Name: opts.Name + "Mutation",
			Fields: graphql.Fields{
				"create": &graphql.Field{
					Type:        modelType,
					Description: "Create",
					Args:        createArgs,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return opts.Service.Create(p.Args)
					},
				},
				"update": &graphql.Field{
					Type:        modelType,
					Description: "Update",
					Args:        createArgs,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id, ok := p.Args[pkCol].(int64)
						if ok {
							return opts.Service.Patch(id, p.Args)
						}
						return nil, nil
					},
				},
				"delete": &graphql.Field{
					Type:        graphql.String,
					Description: "Delete",
					Args: graphql.FieldConfigArgument{
						pkCol: &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id, ok := p.Args[pkCol].(int)
						if ok {
							return "", opts.Service.Remove(int64(id))
						}
						return nil, nil
					},
				},
			},
		},
	)
	return queryType, mutationType
}
