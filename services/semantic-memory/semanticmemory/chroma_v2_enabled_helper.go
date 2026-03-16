//go:build chroma_v2_enabled
// +build chroma_v2_enabled

// Package semanticmemory
package semanticmemory

// createChromaV2Client creates a ChromaV2Client if the build tag is enabled
func createChromaV2Client() (SemanticStorage, error) {
	return NewChromaV2Client()
}