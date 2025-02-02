package messagetemplate

import "time"

func New(template string) MessageTemplate {
	return &messageTemplate{
		id:        0,
		template:  template,
		createdAt: time.Now(),
	}
}

func NewWithID(id uint, template string, createdAt time.Time) MessageTemplate {
	return &messageTemplate{
		id:        id,
		template:  template,
		createdAt: createdAt,
	}
}

type messageTemplate struct {
	id        uint
	template  string
	createdAt time.Time
}

func (m *messageTemplate) ID() uint {
	return m.id
}

func (m *messageTemplate) Template() string {
	return m.template
}

func (m *messageTemplate) UpdateTemplate(template string) MessageTemplate {
	return &messageTemplate{
		id:        m.id,
		template:  template,
		createdAt: m.createdAt,
	}
}

func (m *messageTemplate) CreatedAt() time.Time {
	return m.createdAt
}
