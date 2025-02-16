package migrations

type Store struct {
	path string
}

func NewStore(path string) (*Store, error) {
	return &Store{path: path}, nil
}
