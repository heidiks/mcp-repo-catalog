package provider

type Registry struct {
	providers []Provider
}

func NewRegistry(providers ...Provider) *Registry {
	return &Registry{providers: providers}
}

func (r *Registry) AllProviders() []Provider {
	return r.providers
}

func (r *Registry) GetByName(name string) Provider {
	for _, p := range r.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}
