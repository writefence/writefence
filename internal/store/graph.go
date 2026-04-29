package store

// Node is the minimal graph entity shape returned by graph-aware stores.
type Node struct {
	ID      string
	Label   string
	Summary string
}

// GraphStore is implemented by stores that can traverse graph neighbors.
// Rules can test this interface at runtime and degrade gracefully when a
// backend only supports the base Store contract.
type GraphStore interface {
	QueryNeighbors(nodeID string, hops int, filter string) ([]Node, error)
}
