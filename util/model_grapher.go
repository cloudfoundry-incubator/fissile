package util

// ModelGrapher is an interface to emit dependency graphs of the various
// elements for troubleshooting purposes.
// It must accept emitting the same node / edge multiple times.
type ModelGrapher interface {
	// GraphNode inserts a new node into the graph
	GraphNode(nodeName string, attrs map[string]string) error
	// GraphEdge inserts a new edge into the graph
	GraphEdge(fromNode, toNode string, attrs map[string]string) error
}
