package dialogue

type Start struct {
	Message string
	Model   string
}

func NewDialogue(message string, model *string) *Start {
	m := &Start{
		Message: message,
	}
	if model != nil {
		m.Model = *model
	} else {
		m.Model = "gpt-4o-2024-05-13"
	}
	return m
}
