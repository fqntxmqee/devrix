package contracts

// TierResolver resolves model tier aliases to concrete model names.
type TierResolver interface {
	ResolveTier(tier string) (string, error)
}
