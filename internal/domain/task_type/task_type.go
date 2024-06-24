package task_type

import "time"

type TaskType struct {
	Id          int64
	Icon        *string
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (t *TaskType) ToGraph() {
}
