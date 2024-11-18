package mapping

// MapViewModels maps entities to view models
func MapViewModels[T any, V any](
	entities []T,
	mapFunc func(T) *V,
) []*V {
	viewModels := make([]*V, len(entities))
	for i, entity := range entities {
		viewModels[i] = mapFunc(entity)
	}
	return viewModels
}

func MapDbModels[T any, V any](
	entities []T,
	mapFunc func(T) (*V, error),
) ([]*V, error) {
	viewModels := make([]*V, len(entities))
	for i, entity := range entities {
		viewModel, err := mapFunc(entity)
		if err != nil {
			return nil, err
		}
		viewModels[i] = viewModel
	}
	return viewModels, nil
}
