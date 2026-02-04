package context

import (
	"sort"
	"sync"
)

// ContextGraph is an immutable graph of context blocks with relationship tracking.
// It stores blocks by content-addressed hash and tracks derivation/reference relationships.
type ContextGraph struct {
	mu sync.RWMutex

	// blocks stores all blocks indexed by hash.
	blocks map[string]ContextBlock

	// derivedFrom tracks provenance: blockHash -> parent BlockRefs.
	derivedFrom map[string][]BlockRef

	// references tracks citations: blockHash -> referenced blockHashes.
	references map[string][]string
}

// NewContextGraph creates a new empty context graph.
func NewContextGraph() *ContextGraph {
	return &ContextGraph{
		blocks:      make(map[string]ContextBlock),
		derivedFrom: make(map[string][]BlockRef),
		references:  make(map[string][]string),
	}
}

// AddBlock adds a block to the graph. Idempotent: if block already exists, this is a no-op.
func (g *ContextGraph) AddBlock(block ContextBlock, derivedFrom []BlockRef, references []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Skip if already exists
	if _, exists := g.blocks[block.Hash]; exists {
		return
	}

	// Add block
	g.blocks[block.Hash] = block

	// Add derivation edges
	if len(derivedFrom) > 0 {
		g.derivedFrom[block.Hash] = derivedFrom
	}

	// Add reference edges
	if len(references) > 0 {
		g.references[block.Hash] = references
	}
}

// RemoveBlock removes a block from the graph. Returns true if block was removed.
func (g *ContextGraph) RemoveBlock(hash string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	_, existed := g.blocks[hash]
	if !existed {
		return false
	}

	delete(g.blocks, hash)
	delete(g.derivedFrom, hash)
	delete(g.references, hash)

	return true
}

// GetBlock retrieves a block by hash. Returns nil if not found.
func (g *ContextGraph) GetBlock(hash string) *ContextBlock {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if block, exists := g.blocks[hash]; exists {
		return &block
	}
	return nil
}

// HasBlock checks if a block exists in the graph.
func (g *ContextGraph) HasBlock(hash string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	_, exists := g.blocks[hash]
	return exists
}

// GetAllBlocks returns all blocks in the graph (unordered).
func (g *ContextGraph) GetAllBlocks() []ContextBlock {
	g.mu.RLock()
	defer g.mu.RUnlock()

	blocks := make([]ContextBlock, 0, len(g.blocks))
	for _, block := range g.blocks {
		blocks = append(blocks, block)
	}

	return blocks
}

// GetBlockCount returns the number of blocks in the graph.
func (g *ContextGraph) GetBlockCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return len(g.blocks)
}

// GetDerivedFrom returns the parent blocks for a given block.
func (g *ContextGraph) GetDerivedFrom(hash string) []BlockRef {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if parents, exists := g.derivedFrom[hash]; exists {
		return parents
	}
	return []BlockRef{}
}

// GetReferences returns the referenced block hashes for a given block.
func (g *ContextGraph) GetReferences(hash string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if refs, exists := g.references[hash]; exists {
		return refs
	}
	return []string{}
}

// Select returns blocks matching a query in deterministic order.
func (g *ContextGraph) Select(query BlockQuery) []ContextBlock {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var matches []ContextBlock
	for _, block := range g.blocks {
		if query.Matches(block) {
			matches = append(matches, block)
		}
	}

	// Sort by kind order, then by hash for determinism
	sort.Slice(matches, func(i, j int) bool {
		kindI := kindOrderIndex(matches[i].Meta.Kind)
		kindJ := kindOrderIndex(matches[j].Meta.Kind)

		if kindI != kindJ {
			return kindI < kindJ
		}

		return matches[i].Hash < matches[j].Hash
	})

	return matches
}

// Clear removes all blocks and edges from the graph.
func (g *ContextGraph) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.blocks = make(map[string]ContextBlock)
	g.derivedFrom = make(map[string][]BlockRef)
	g.references = make(map[string][]string)
}

// GetStats returns graph statistics.
func (g *ContextGraph) GetStats() GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return GraphStats{
		BlockCount:          len(g.blocks),
		DerivationEdgeCount: len(g.derivedFrom),
		ReferenceEdgeCount:  len(g.references),
	}
}

// GraphStats contains statistics about the context graph.
type GraphStats struct {
	BlockCount          int
	DerivationEdgeCount int
	ReferenceEdgeCount  int
}

// kindOrderIndex returns the index of a kind in KindOrder, or len(KindOrder) if not found.
func kindOrderIndex(kind BlockKind) int {
	for i, k := range KindOrder {
		if k == kind {
			return i
		}
	}
	return len(KindOrder)
}
