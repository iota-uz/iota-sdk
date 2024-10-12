package permission

import "errors"

type Resource string
type Action string

func NewResource(r string) (Resource, error) {
	resource := Resource(r)
	if !resource.IsValid() {
		return "", errors.New("invalid resource")
	}
	return resource, nil
}

func NewAction(a string) (Action, error) {
	action := Action(a)
	if !action.IsValid() {
		return "", errors.New("invalid action")
	}
	return action, nil
}

const (
	ResourcePayment Resource = "payment"
	ResourceUser    Resource = "user"
	ResourceRole    Resource = "role"
	ResourceAccount Resource = "account"
	ResourceStage   Resource = "stage"
	ResourceProject Resource = "project"
)

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

func (r Resource) IsValid() bool {
	switch r {
	case ResourcePayment, ResourceUser, ResourceRole, ResourceAccount, ResourceStage, ResourceProject:
		return true
	}
	return false
}

func (a Action) IsValid() bool {
	switch a {
	case ActionCreate, ActionRead, ActionUpdate, ActionDelete:
		return true
	}
	return false
}
