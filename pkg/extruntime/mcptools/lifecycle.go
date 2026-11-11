package mcptools

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

type publishIn struct {
	Namespace string `json:"namespace"`
	Version   string `json:"version,omitempty" jsonschema:"version label; taken from package.json when omitted"`
}

type versionIn struct {
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
}

type versionList struct {
	Versions []manage.Version `json:"versions"`
}

type activationList struct {
	Activations []manage.Activation `json:"activations"`
}

type reviewIn struct {
	Namespace          string `json:"namespace"`
	Version            string `json:"version"`
	Approve            bool   `json:"approve"`
	Comment            string `json:"comment,omitempty"`
	ExpectSourceHash   string `json:"expect_source_hash,omitempty" jsonschema:"source hash the reviewer looked at; the review is refused when the version differs"`
	ExpectArtifactHash string `json:"expect_artifact_hash,omitempty" jsonschema:"artifact hash the reviewer looked at"`
}

type bindingsIn struct {
	History bool `json:"history,omitempty" jsonschema:"include closed bindings"`
}

type bindingList struct {
	Bindings []manage.Binding `json:"bindings"`
}

type bindIn struct {
	SlotKind string `json:"slot_kind" jsonschema:"command, screen or shell"`
	TargetID string `json:"target_id" jsonschema:"contract ID of the command, or the route ID of the screen"`
	Package  string `json:"package" jsonschema:"namespace of the implementing package"`
	Version  string `json:"version"`
	Export   string `json:"export" jsonschema:"export of the package that implements the slot"`
}

type unbindIn struct {
	SlotKind string `json:"slot_kind"`
	TargetID string `json:"target_id"`
}

type ownerList struct {
	Owners []manage.TriggerOwner `json:"owners"`
}

type ownerAssignIn struct {
	TriggerID string `json:"trigger_id" jsonschema:"ID of a subscription, schedule or endpoint of an active package"`
	UserID    int64  `json:"user_id" jsonschema:"user the trigger acts as when it has no initiating user"`
}

type ownerRemoveIn struct {
	TriggerID string `json:"trigger_id"`
}

type auditIn struct {
	Namespace string `json:"namespace,omitempty"`
	Action    string `json:"action,omitempty"`
	BeforeID  int64  `json:"before_id,omitempty" jsonschema:"page: only rows with a smaller id"`
	Limit     int    `json:"limit,omitempty"`
}

type auditList struct {
	Entries []manage.AuditEntry `json:"entries"`
}

func lifecycleTools(r *registrar, d Deps) {
	svc := d.Manage

	add(r, mcp.Spec{
		Name: "ext_publish",
		Description: describe("Build the saved draft and store it as an immutable version. Refused with the "+
			"diagnostics when the build fails. A published version is not live until it is reviewed and activated.", manage.PermPublish),
		Annotations: mcp.Annotations{Title: "Publish version"},
		Policy:      managePolicy(), Meta: meta(manage.PermPublish),
	}, func(ctx context.Context, actor spi.Subject, in publishIn) (manage.VersionDetail, error) {
		return svc.Publish(ctx, actor, in.Namespace, in.Version)
	})

	add(r, mcp.Spec{
		Name:        "ext_version_list",
		Description: describe("List published versions of a package with hashes, review state and which one is active.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "List versions", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in namespaceIn) (versionList, error) {
		out, err := svc.ListVersions(ctx, actor, in.Namespace)
		return versionList{Versions: nonNilSlice(out)}, err
	})

	add(r, mcp.Spec{
		Name:        "ext_version_get",
		Description: describe("Read one published version including its manifest (declared resources, implementations, triggers).", manage.PermRead),
		Annotations: mcp.Annotations{Title: "Get version", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in versionIn) (manage.VersionDetail, error) {
		return svc.GetVersion(ctx, actor, in.Namespace, in.Version)
	})

	add(r, mcp.Spec{
		Name: "ext_review",
		Description: describe("Approve or reject a published version. The decision is bound to the exact source and artifact "+
			"hashes; pass expect_* to make sure the reviewed bytes are the ones being approved. Self-review is allowed "+
			"for holders of the permission and recorded as such.", manage.PermReview),
		Annotations: mcp.Annotations{Title: "Review version"},
		Policy:      managePolicy(), Meta: meta(manage.PermReview),
	}, func(ctx context.Context, actor spi.Subject, in reviewIn) (manage.Review, error) {
		return svc.ReviewExact(ctx, actor, in.Namespace, in.Version, manage.ReviewInput{
			Approve: in.Approve, Comment: in.Comment, ExpectSourceHash: in.ExpectSourceHash, ExpectArtifactHash: in.ExpectArtifactHash,
		})
	})

	add(r, mcp.Spec{
		Name: "ext_activate",
		Description: describe("Make an approved version the live one for the whole application: applies its schema, binds "+
			"its implementations and starts its triggers. One active version per package.", manage.PermPublish),
		Annotations: mcp.Annotations{Title: "Activate version", Destructive: true},
		Policy:      managePolicy(), Meta: meta(manage.PermPublish),
	}, func(ctx context.Context, actor spi.Subject, in versionIn) (manage.ActivationResult, error) {
		return svc.Activate(ctx, actor, in.Namespace, in.Version)
	})

	add(r, mcp.Spec{
		Name:        "ext_deactivate",
		Description: describe("Stop the active version of a package; its data is kept.", manage.PermPublish),
		Annotations: mcp.Annotations{Title: "Deactivate package", Destructive: true, Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermPublish),
	}, func(ctx context.Context, actor spi.Subject, in namespaceIn) (ack, error) {
		return done, svc.Deactivate(ctx, actor, in.Namespace)
	})

	add(r, mcp.Spec{
		Name:        "ext_purge",
		Description: describe("Delete the data of a deactivated package. This cannot be undone.", manage.PermOperate),
		Annotations: mcp.Annotations{Title: "Purge package data", Destructive: true},
		Policy:      managePolicy(), Meta: meta(manage.PermOperate),
	}, func(ctx context.Context, actor spi.Subject, in namespaceIn) (ack, error) {
		return done, svc.Purge(ctx, actor, in.Namespace)
	})

	add(r, mcp.Spec{
		Name:        "ext_activation_list",
		Description: describe("List the activation history of a package.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "List activations", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in namespaceIn) (activationList, error) {
		out, err := svc.ListActivations(ctx, actor, in.Namespace)
		return activationList{Activations: nonNilSlice(out)}, err
	})

	add(r, mcp.Spec{
		Name:        "ext_binding_list",
		Description: describe("List which implementation serves each command, screen and shell slot.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "List bindings", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in bindingsIn) (bindingList, error) {
		out, err := svc.ListBindings(ctx, actor, in.History)
		return bindingList{Bindings: nonNilSlice(out)}, err
	})

	add(r, mcp.Spec{
		Name: "ext_bind",
		Description: describe("Replace the implementation of a command or screen, or the shell, with an export of an "+
			"active package version. One binding per slot application-wide.", manage.PermPublish),
		Annotations: mcp.Annotations{Title: "Bind slot", Destructive: true},
		Policy:      managePolicy(), Meta: meta(manage.PermPublish),
	}, func(ctx context.Context, actor spi.Subject, in bindIn) (manage.Binding, error) {
		return svc.Bind(ctx, actor, in.SlotKind, in.TargetID, in.Package, in.Version, in.Export)
	})

	add(r, mcp.Spec{
		Name:        "ext_unbind",
		Description: describe("Release a replacement binding; the slot returns to its previous implementation.", manage.PermPublish),
		Annotations: mcp.Annotations{Title: "Unbind slot", Destructive: true, Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermPublish),
	}, func(ctx context.Context, actor spi.Subject, in unbindIn) (ack, error) {
		return done, svc.Unbind(ctx, actor, in.SlotKind, in.TargetID)
	})

	add(r, mcp.Spec{
		Name:        "ext_owner_list",
		Description: describe("List the users assigned as owners of triggers without an initiating user (schedules, subscriptions, endpoints).", manage.PermRead),
		Annotations: mcp.Annotations{Title: "List trigger owners", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, _ struct{}) (ownerList, error) {
		out, err := svc.ListOwners(ctx, actor)
		return ownerList{Owners: nonNilSlice(out)}, err
	})

	add(r, mcp.Spec{
		Name:        "ext_owner_assign",
		Description: describe("Assign the user a trigger acts as in your tenant. The trigger runs with that user's current rights.", manage.PermPublish),
		Annotations: mcp.Annotations{Title: "Assign trigger owner", Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermPublish),
	}, func(ctx context.Context, actor spi.Subject, in ownerAssignIn) (manage.TriggerOwner, error) {
		return svc.AssignOwner(ctx, actor, in.TriggerID, in.UserID)
	})

	add(r, mcp.Spec{
		Name:        "ext_owner_remove",
		Description: describe("Remove a trigger owner; the trigger's runs are blocked until a new owner is assigned.", manage.PermPublish),
		Annotations: mcp.Annotations{Title: "Remove trigger owner", Destructive: true, Idempotent: true},
		Policy:      managePolicy(), Meta: meta(manage.PermPublish),
	}, func(ctx context.Context, actor spi.Subject, in ownerRemoveIn) (ack, error) {
		return done, svc.RemoveOwner(ctx, actor, in.TriggerID)
	})

	add(r, mcp.Spec{
		Name:        "ext_audit_list",
		Description: describe("Read the management audit log, newest first: who did what to which version with which hashes, including refused attempts.", manage.PermRead),
		Annotations: mcp.Annotations{Title: "Audit log", ReadOnly: true, Idempotent: true},
		Policy:      readPolicy(manage.PermRead), Meta: meta(manage.PermRead),
	}, func(ctx context.Context, actor spi.Subject, in auditIn) (auditList, error) {
		out, err := svc.ListAudit(ctx, actor, manage.AuditFilter{Namespace: in.Namespace, Action: in.Action, BeforeID: in.BeforeID, Limit: in.Limit})
		return auditList{Entries: nonNilSlice(out)}, err
	})
}
