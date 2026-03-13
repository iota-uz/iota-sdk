// Package messagetemplate provides this package.
package messagetemplate

import "time"

type MessageTemplate interface {
	ID() uint
	Template() string
	UpdateTemplate(template string) MessageTemplate
	CreatedAt() time.Time
}
