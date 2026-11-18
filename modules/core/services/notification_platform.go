package services

import (
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
)

// NotificationPlatform bundles the pluggable parts of the recipient/target
// model: where notice identities are stored, how content is rendered, how
// records and users become channel targets, and which access and business
// policies apply. The routing, dispatch and channel services share one instance.
type NotificationPlatform struct {
	Notices   notifications.NoticeRepository
	Renderers *notifications.ContentRenderers
	// Contacts resolves record recipients; entities without a resolver are
	// rejected when the notice is enqueued.
	Contacts *notifications.ContactResolvers
	// Devices is the push device registry of user recipients.
	Devices notifications.PushDeviceRepository
	// SubjectAccess is checked for user recipients when a notice has a subject.
	// Nil allows every user.
	SubjectAccess notifications.SubjectAccess
	// Policy is the business delivery policy for record and address
	// recipients. Nil allows every delivery.
	Policy notifications.DeliveryPolicy
}

func (p *NotificationPlatform) policy() notifications.DeliveryPolicy {
	if p == nil || p.Policy == nil {
		return notifications.AllowAllPolicy{}
	}
	return p.Policy
}
