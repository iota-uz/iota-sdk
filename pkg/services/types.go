package services

type Service interface {
	Get(id string) (interface{}, error)
	GetAll() ([]interface{}, error)
	PaginatedList(offset int, limit int) ([]interface{}, error)
	Create(input interface{}) (interface{}, error)
	Update(id string, input interface{}) (interface{}, error)
	Delete(id string) error
}
