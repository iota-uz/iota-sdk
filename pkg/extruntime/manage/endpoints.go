package manage

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/endpoint"
)

// EndpointSource lists the endpoints of the activated versions; it
// implements endpoint.Source from the cached active catalog.
func (s *Service) EndpointSource() endpoint.Source { return endpointSource{s} }

type endpointSource struct{ s *Service }

func (src endpointSource) ActiveEndpoints(ctx context.Context) ([]endpoint.Endpoint, error) {
	s := src.s
	active, err := s.catalog.Active(ctx)
	if err != nil {
		return nil, err
	}
	var out []endpoint.Endpoint
	for _, av := range active {
		if len(av.Manifest.Endpoints) == 0 {
			continue
		}
		art, secrets, err := s.ResolveArtifact(ctx, av.Namespace, av.Version)
		if err != nil {
			return nil, err
		}
		for _, ep := range av.Manifest.Endpoints {
			out = append(out, endpoint.Endpoint{
				ID: ep.ID, Package: av.Namespace, Version: av.Version, Export: ep.Export,
				Method: ep.Method, Path: ep.Path, Public: ep.Public, Artifact: art, Secrets: secrets,
			})
		}
	}
	return out, nil
}
