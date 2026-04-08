package composition

type AppletEngineBuilder struct {
	backends *BackendRegistry
}

func NewAppletEngineBuilder() *AppletEngineBuilder {
	return &AppletEngineBuilder{
		backends: NewBackendRegistry(),
	}
}

func (b *AppletEngineBuilder) Backends() *BackendRegistry {
	if b == nil {
		return nil
	}
	return b.backends
}
