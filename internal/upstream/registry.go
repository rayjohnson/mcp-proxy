package upstream

import "fmt"

var adapters = map[string]Adapter{}

// Register adds an adapter for a server type. Called from init() in each adapter file.
func Register(serverType string, a Adapter) {
	adapters[serverType] = a
}

// GetAdapter returns the adapter for the given server type.
func GetAdapter(serverType string) (Adapter, error) {
	a, ok := adapters[serverType]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for server type %q", serverType)
	}
	return a, nil
}
