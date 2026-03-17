//go:build !chroma_v2_enabled
// +build !chroma_v2_enabled

// Package semanticmemory
package semanticmemory

import "fmt"

// createChromaV2Client creates a ChromaV2Client if the build tag is enabled
func createChromaV2Client() (SemanticStorage, error) {
	// This function will only be compiled when chroma_v2_enabled tag is NOT present
	// It returns an error indicating that v2 is not supported in this build
	return nil, fmt.Errorf("ChromaDB v2 client not available in this build - compile with -tags chroma_v2_enabled")
}