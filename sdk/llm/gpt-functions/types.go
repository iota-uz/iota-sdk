package functions

type ChatFunctionDefinition interface {
	Name() string
	Description() string
	Arguments() map[string]interface{}
	Execute(args map[string]interface{}) (string, error)
}
