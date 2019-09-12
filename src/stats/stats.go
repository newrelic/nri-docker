package stats

type Provider interface {
	Fetch(containerID string) (Cooked, error)
}
