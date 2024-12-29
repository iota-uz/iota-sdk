package tasktype

import "time"

type TaskType struct {
	ID          int64
	Icon        *string
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (t *TaskType) ToGraph() {
}
