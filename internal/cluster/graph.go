package cluster

import (
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceNode struct {
	UID    string
	GVR    schema.GroupVersionResource
	Object *unstructured.Unstructured
}

type Graph struct {
	nodes map[string]*ResourceNode
	edges map[string][]string // UID -> []UID
	mu    sync.RWMutex
}

func (g *Graph) OnAdd(obj interface{}) {
	u := obj.(*unstructured.Unstructured)

	g.mu.Lock()
	defer g.mu.Unlock()

	// 1. Add Node
	node := &ResourceNode{UID: string(u.GetUID()), Object: u}
	g.nodes[node.UID] = node

	// 2. Build Edges based on OwnerReferences
	for _, ref := range u.GetOwnerReferences() {
		g.edges[string(ref.UID)] = append(g.edges[string(ref.UID)], node.UID)
	}
}
