package permission

import "errors"

type Resource string
type Action string
type Modifier string

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

func NewModifier(m string) (Modifier, error) {
	modifier := Modifier(m)
	if !modifier.IsValid() {
		return "", errors.New("invalid modifier")
	}
	return modifier, nil
}

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

func (m Modifier) IsValid() bool {
	switch m {
	case ModifierAll, ModifierOwn:
		return true
	}
	return false
}
