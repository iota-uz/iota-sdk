package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// PolicyName is the name under which the adapter registers its composite
// policy on notifications.DeliveryPolicies.
const PolicyName = "extruntime"

// PolicyRequest is the wire request a package delivery policy receives (the
// second argument of the handler bound with implement(deliveryPolicy, ...)).
// It is a public contract: fields are only ever added.
//
//	{
//	  "notice":    { "key": "acme.renewal.v1", "version": 1, "snapshot": "acme@1.4.0" },
//	  "subject":   { "entity": "acme.policy", "key": "P-1" } | null,
//	  "recipient": { "kind": "record", "ref": { "entity": "acme.customer", "key": "c-1" }, "address": null },
//	  "channel":   "sms" | "email" | "push",
//	  "target":    { "address": "+998901234567", "deviceId": null }
//	}
type PolicyRequest struct {
	Notice    PolicyNotice    `json:"notice"`
	Subject   *spi.ObjectRef  `json:"subject"`
	Recipient PolicyRecipient `json:"recipient"`
	Channel   string          `json:"channel"`
	Target    PolicyTarget    `json:"target"`
}

// PolicyNotice identifies the notice being delivered: its catalog key and
// version (the key ends in ".v<version>") and the pinned snapshot
// ("package@version") that rendered it.
type PolicyNotice struct {
	Key      string `json:"key"`
	Version  int    `json:"version"`
	Snapshot string `json:"snapshot"`
}

// PolicyRecipient is who the notice is for. Kind is "record" (Ref set,
// Address null) or "address" (Address set, Ref null). Package policies are
// evaluated for record recipients of their entity only, so they receive
// "record" today; handlers must still allow kinds they do not know.
type PolicyRecipient struct {
	Kind    string         `json:"kind"`
	Ref     *spi.ObjectRef `json:"ref"`
	Address *string        `json:"address"`
}

// PolicyTarget is the concrete address the delivery would go to; DeviceID is
// set for a registered push device.
type PolicyTarget struct {
	Address  string  `json:"address"`
	DeviceID *string `json:"deviceId"`
}

// PolicyDecision is the wire answer of a package delivery policy:
// {"allow": true} or {"allow": false, "reason": "no_consent"}. Reason is an
// optional machine code ([a-z0-9_.-], at most 64 characters) that is stored
// with the skipped delivery as "policy_denied:<reason>"; it must not carry
// personal data. Allow must be present and boolean, anything else is a policy
// error and the delivery is retried rather than sent unvetted.
type PolicyDecision struct {
	Allow  *bool  `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

var reasonPattern = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)

// deliveryPolicy is the composite policy of every active package: for a record
// recipient it runs the deliveryPolicy implementations active at that moment
// whose contract is bound to the recipient's entity, as the trigger owner of
// each. Implementations are found per evaluation, so a deactivated package
// stops being consulted without any unregistration.
type deliveryPolicy struct{ a *Adapter }

type policyImpl struct {
	policyID string
	pkg      string
	version  string
	export   string
}

// find lists the active implementations of policies for an entity in a fixed
// order (contract ID, then package).
func (p *deliveryPolicy) find(ctx context.Context, entity string) ([]policyImpl, error) {
	active, err := p.a.cfg.Catalog.Active(ctx)
	if err != nil {
		return nil, err
	}
	var found []policyImpl
	for _, av := range active {
		for _, i := range av.Manifest.Implementations {
			if i.Kind != manifest.ImplDeliveryPolicy {
				continue
			}
			res, ok, err := p.a.cfg.Catalog.Resolve(ctx, i.Implements)
			if err != nil {
				return nil, err
			}
			if ok && res.Resource.Kind == manifest.ResDeliveryPolicy && res.Resource.Subject == entity {
				found = append(found, policyImpl{policyID: i.Implements, pkg: av.Namespace, version: av.Version, export: i.Export})
			}
		}
	}
	slices.SortFunc(found, func(x, y policyImpl) int {
		if c := strings.Compare(x.policyID, y.policyID); c != 0 {
			return c
		}
		return strings.Compare(x.pkg, y.pkg)
	})
	return found, nil
}

// Evaluate implements notifications.DeliveryPolicy. The first denial wins.
func (p *deliveryPolicy) Evaluate(ctx context.Context, req notifications.PolicyRequest) (notifications.PolicyDecision, error) {
	const op = serrors.Op("extruntime.notify.deliveryPolicy.Evaluate")
	allow := notifications.PolicyDecision{Allow: true}
	if req.Recipient.Kind != notifications.RecipientRecord || req.Recipient.Record == nil {
		return allow, nil
	}
	impls, err := p.find(ctx, req.Recipient.Record.Entity)
	if err != nil {
		return notifications.PolicyDecision{}, serrors.Wrap(op, err)
	}
	for _, impl := range impls {
		decision, err := p.run(ctx, impl, req)
		if err != nil {
			return notifications.PolicyDecision{}, serrors.Wrap(op, err)
		}
		if !decision.Allow {
			return decision, nil
		}
	}
	return allow, nil
}

func wireRequest(req notifications.PolicyRequest) PolicyRequest {
	out := PolicyRequest{
		Notice:    PolicyNotice{Key: req.EventKey, Version: req.EventVersion, Snapshot: req.SnapshotRef},
		Channel:   templateChannel(req.Target.Channel),
		Target:    PolicyTarget{Address: req.Target.Address},
		Recipient: PolicyRecipient{Kind: string(req.Recipient.Kind)},
	}
	if !req.Subject.IsZero() {
		out.Subject = &spi.ObjectRef{Entity: req.Subject.Entity, Key: req.Subject.Key}
	}
	if req.Recipient.Record != nil {
		out.Recipient.Ref = &spi.ObjectRef{Entity: req.Recipient.Record.Entity, Key: req.Recipient.Record.Key}
	}
	if req.Recipient.Kind == notifications.RecipientAddress && req.Target.Address != "" {
		address := req.Target.Address
		out.Recipient.Address = &address
	}
	if req.Target.DeviceID != "" {
		id := req.Target.DeviceID
		out.Target.DeviceID = &id
	}
	return out
}

// run executes one implementation as the owner of its policy contract. A
// policy without an owner, with a failing handler or with a malformed answer
// is an error (the delivery is retried), never an implicit allow.
func (p *deliveryPolicy) run(ctx context.Context, impl policyImpl, req notifications.PolicyRequest) (notifications.PolicyDecision, error) {
	const op = serrors.Op("extruntime.notify.deliveryPolicy.run")
	if p.a.cfg.Owners == nil || p.a.cfg.Subjects == nil {
		return notifications.PolicyDecision{}, serrors.New(serrors.FailedPrecondition, "delivery policies need trigger owners").WithOp(op)
	}
	owner, has, err := p.a.cfg.Owners.OwnerOf(ctx, req.TenantID, impl.policyID)
	if err != nil {
		return notifications.PolicyDecision{}, serrors.Wrap(op, err)
	}
	if !has {
		return notifications.PolicyDecision{}, serrors.New(serrors.FailedPrecondition, "no owner is assigned to delivery policy "+impl.policyID).WithOp(op)
	}
	subject, err := p.a.cfg.Subjects.ResolveSubject(ctx, req.TenantID, owner)
	if err != nil {
		return notifications.PolicyDecision{}, serrors.Wrap(op, err)
	}
	subject.Kind = spi.SubjectOwner
	art, secrets, err := p.a.cfg.Artifacts.ResolveArtifact(ctx, impl.pkg, impl.version)
	if err != nil {
		return notifications.PolicyDecision{}, serrors.Wrap(op, err)
	}
	input, err := json.Marshal(wireRequest(req))
	if err != nil {
		return notifications.PolicyDecision{}, serrors.Wrap(op, err)
	}
	res, err := p.a.cfg.Runner.Run(ctx, host.RunRequest{
		Export: host.Export{
			Kind: pool.KindQuery, Resource: impl.policyID,
			Binding: host.Binding{Contract: impl.policyID, Package: impl.pkg, Version: impl.version, Export: impl.export, Artifact: art, Secrets: secrets},
		},
		Input: input, Subject: subject,
		Trigger: host.Trigger{Kind: host.TriggerSubscription, Ref: "policy:" + impl.policyID},
	})
	if err != nil {
		return notifications.PolicyDecision{}, serrors.Wrap(op, err)
	}
	return decodeDecision(impl.policyID, res.Output)
}

// decodeDecision parses the policy's answer strictly.
func decodeDecision(policyID string, raw json.RawMessage) (notifications.PolicyDecision, error) {
	const op = serrors.Op("extruntime.notify.decodeDecision")
	var out PolicyDecision
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil || out.Allow == nil {
		return notifications.PolicyDecision{}, serrors.New(serrors.Invalid, fmt.Sprintf("delivery policy %s must return {allow: boolean, reason?: string}", policyID)).WithOp(op)
	}
	if *out.Allow {
		return notifications.PolicyDecision{Allow: true}, nil
	}
	reason := ""
	if reasonPattern.MatchString(out.Reason) {
		reason = out.Reason
	}
	return notifications.PolicyDecision{Allow: false, Reason: reason}, nil
}
