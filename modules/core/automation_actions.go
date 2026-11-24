package core

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions/servicectx"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Keys of the core module's automation actions.
const (
	ActionUserBlock       = "core.user.block"
	ActionUserUnblock     = "core.user.unblock"
	ActionGroupAddUser    = "core.group.add_user"
	ActionGroupRemoveUser = "core.group.remove_user"
)

// contributeAutomationActions offers the user and group operations of the core services to automation
// graphs: they run as the run-as user, with its rights, through the same services the UI calls.
func contributeAutomationActions(b *composition.Builder) {
	actions.ContributeFunc(b, func(c *composition.Container) ([]actions.Action, error) {
		pool, err := composition.Resolve[*pgxpool.Pool](c)
		if err != nil {
			return nil, err
		}
		users, err := composition.Resolve[user.Repository](c)
		if err != nil {
			return nil, err
		}
		userService, err := composition.Resolve[*services.UserService](c)
		if err != nil {
			return nil, err
		}
		groupService, err := composition.Resolve[*services.GroupService](c)
		if err != nil {
			return nil, err
		}
		env := coreActionEnv{pool: pool, users: users}
		return []actions.Action{
			userBlockAction{env: env, users: userService},
			userUnblockAction{env: env, users: userService},
			groupMemberAction{env: env, groups: groupService, add: true},
			groupMemberAction{env: env, groups: groupService, add: false},
		}, nil
	})
}

type coreActionEnv struct {
	pool  *pgxpool.Pool
	users user.Repository
}

func (e coreActionEnv) ctx(ctx context.Context, actor spi.Subject) (context.Context, error) {
	return servicectx.Context(ctx, e.pool, e.users, actor)
}

var (
	userIDProp = manifest.Property{Name: "user_id", Type: manifest.Type{Kind: manifest.KindInt}}
	userOutput = &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		{Name: "user_id", Type: manifest.Type{Kind: manifest.KindInt}},
		{Name: "blocked", Type: manifest.Type{Kind: manifest.KindBool}},
	}}
)

func userOutputOf(u user.User) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"user_id": u.ID(), "blocked": u.IsBlocked()})
}

type userBlockAction struct {
	env   coreActionEnv
	users *services.UserService
}

func (userBlockAction) Key() string { return ActionUserBlock }
func (userBlockAction) Spec() actions.ActionSpec {
	return actions.ActionSpec{
		Label: manifest.Label{"en": "Block user", "ru": "Заблокировать пользователя", "uz": "Foydalanuvchini bloklash", "uz-Cyrl": "Фойдаланувчини блоклаш"},
		Description: manifest.Label{
			"en": "Blocks a user with a reason; the user can no longer sign in.", "ru": "Блокирует пользователя с причиной; он больше не сможет войти.",
			"uz": "Foydalanuvchini sabab bilan bloklaydi; u endi tizimga kira olmaydi.", "uz-Cyrl": "Фойдаланувчини сабаб билан блоклайди; у энди тизимга кира олмайди.",
		},
		Group: "users",
		Args: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			userIDProp, {Name: "reason", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 1024}},
		}},
		Output: userOutput, Permission: permissions.UserUpdateBlockStatus.Name(),
	}
}

func (a userBlockAction) Run(ctx context.Context, actor spi.Subject, args json.RawMessage) (string, json.RawMessage, error) {
	var in struct {
		UserID uint   `json:"user_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", nil, actions.ErrInvalidArgs
	}
	sctx, err := a.env.ctx(ctx, actor)
	if err != nil {
		return "", nil, err
	}
	u, err := a.users.BlockUser(sctx, in.UserID, in.Reason)
	if err != nil {
		return "", nil, servicectx.Refusal(err)
	}
	out, err := userOutputOf(u)
	return actions.OutcomeDone, out, err
}

type userUnblockAction struct {
	env   coreActionEnv
	users *services.UserService
}

func (userUnblockAction) Key() string { return ActionUserUnblock }
func (userUnblockAction) Spec() actions.ActionSpec {
	return actions.ActionSpec{
		Label: manifest.Label{"en": "Unblock user", "ru": "Разблокировать пользователя", "uz": "Foydalanuvchini blokdan chiqarish", "uz-Cyrl": "Фойдаланувчини блокдан чиқариш"},
		Description: manifest.Label{
			"en": "Unblocks a blocked user.", "ru": "Снимает блокировку с пользователя.",
			"uz": "Bloklangan foydalanuvchini blokdan chiqaradi.", "uz-Cyrl": "Блокланган фойдаланувчини блокдан чиқаради.",
		},
		Group:  "users",
		Args:   &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{userIDProp}},
		Output: userOutput, Permission: permissions.UserUpdateBlockStatus.Name(),
	}
}

func (a userUnblockAction) Run(ctx context.Context, actor spi.Subject, args json.RawMessage) (string, json.RawMessage, error) {
	var in struct {
		UserID uint `json:"user_id"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", nil, actions.ErrInvalidArgs
	}
	sctx, err := a.env.ctx(ctx, actor)
	if err != nil {
		return "", nil, err
	}
	u, err := a.users.UnblockUser(sctx, in.UserID)
	if err != nil {
		return "", nil, servicectx.Refusal(err)
	}
	out, err := userOutputOf(u)
	return actions.OutcomeDone, out, err
}

// groupMemberAction adds a user to a group or removes one from it.
type groupMemberAction struct {
	env    coreActionEnv
	groups *services.GroupService
	add    bool
}

func (a groupMemberAction) Key() string {
	if a.add {
		return ActionGroupAddUser
	}
	return ActionGroupRemoveUser
}

func (a groupMemberAction) Spec() actions.ActionSpec {
	label := manifest.Label{"en": "Add user to group", "ru": "Добавить пользователя в группу", "uz": "Foydalanuvchini guruhga qo‘shish", "uz-Cyrl": "Фойдаланувчини гуруҳга қўшиш"}
	if !a.add {
		label = manifest.Label{"en": "Remove user from group", "ru": "Удалить пользователя из группы", "uz": "Foydalanuvchini guruhdan chiqarish", "uz-Cyrl": "Фойдаланувчини гуруҳдан чиқариш"}
	}
	return actions.ActionSpec{
		Label: label, Group: "users",
		Args: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			{Name: "group_id", Type: manifest.Type{Kind: manifest.KindUUID}}, userIDProp,
		}},
		Output: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			{Name: "group_id", Type: manifest.Type{Kind: manifest.KindUUID}},
			{Name: "members", Type: manifest.Type{Kind: manifest.KindInt}},
		}},
		Permission: permissions.GroupUpdate.Name(),
	}
}

func (a groupMemberAction) Run(ctx context.Context, actor spi.Subject, args json.RawMessage) (string, json.RawMessage, error) {
	var in struct {
		GroupID uuid.UUID `json:"group_id"`
		UserID  uint      `json:"user_id"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", nil, actions.ErrInvalidArgs
	}
	sctx, err := a.env.ctx(ctx, actor)
	if err != nil {
		return "", nil, err
	}
	member, err := a.env.users.GetByID(sctx, in.UserID)
	if err != nil {
		return "", nil, &actions.ActionError{Code: "user_not_found", Message: "the user is not a user of the tenant"}
	}
	change := a.groups.RemoveUser
	if a.add {
		change = a.groups.AddUser
	}
	saved, err := change(sctx, in.GroupID, member)
	if err != nil {
		return "", nil, servicectx.Refusal(err)
	}
	out, err := json.Marshal(map[string]any{"group_id": in.GroupID.String(), "members": len(saved.Users())})
	return actions.OutcomeDone, out, err
}
