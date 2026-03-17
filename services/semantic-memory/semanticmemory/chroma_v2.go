//go:build chroma_v2_enabled
// +build chroma_v2_enabled

// Package semanticmemory handles ChromaDB integration using the official chroma-go client v2.
package semanticmemory

import (
	"context"
	"fmt"
	"log"
	"os"

	v2 "github.com/amikos-tech/chroma-go/pkg/api/v2"
	"github.com/amikos-tech/chroma-go/pkg/embeddings"
	"github.com/amikos-tech/chroma-go/pkg/embeddings/ollama"
)

// ChromaV2Client handles communication with ChromaDB via the official Go client v2.
// Implements the SemanticStorage interface.
type ChromaV2Client struct {
	client         v2.Client
	collectionName string
	collection     v2.Collection // Updated to correct type
	embeddings     *ollama.OllamaEmbeddingFunction
}

// Ensure ChromaV2Client implements SemanticStorage interface
var _ SemanticStorage = (*ChromaV2Client)(nil)

// NewChromaV2Client creates a new ChromaV2Client using the official Go client.
func NewChromaV2Client() (*ChromaV2Client, error) {
	url := os.Getenv("CHROMA_URL")
	if url == "" {
		url = "http://chromadb:8000"
	}

	urlEmbeding := os.Getenv("EMBEDING_URL")
	if urlEmbeding == "" {
		urlEmbeding = "http://qwen3-service:11434"
	}

	modelEmbedding := os.Getenv("EMBEDING_MODEL")
	if modelEmbedding == "" {
		modelEmbedding = "nomic-embed-text:latest"
	}

	client, err := v2.NewHTTPClient(v2.WithBaseURL(url),
		v2.WithDatabaseAndTenant("default_database", "default_tenant"),
		v2.WithDefaultHeaders(map[string]string{"X-Custom-Header": "header-value"}),
		v2.WithDebug())
	if err != nil {
		return nil, fmt.Errorf("failed to create ChromaDB v2 client: %w", err)
	}

	collectionName := os.Getenv("CHROMA_COLLECTION_NAME")
	if collectionName == "" {
		collectionName = "world_memory"
	}

	ef, err := ollama.NewOllamaEmbeddingFunction(
		ollama.WithBaseURL(urlEmbeding),
		ollama.WithModel(embeddings.EmbeddingModel(modelEmbedding)),
	)
	if err != nil {
		log.Fatalf("Error creating embedding function: %v", err)
	}

	documents := []string{
		"Document 1 content here",
		"Document 2 content here",
	}
	resp, err := ef.EmbedDocuments(context.Background(), documents)
	if err != nil {
		fmt.Printf("Error embedding documents: %s \n", err)
	}
	fmt.Printf("Embedding response: %v \n", resp)

	chromaV2Client := &ChromaV2Client{
		client:         client,
		collectionName: collectionName,
		embeddings:     ef,
	}

	// Initialize collection
	err = chromaV2Client.initializeCollection(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize collection: %w", err)
	}

	return chromaV2Client, nil
}

// initializeCollection gets or creates the collection.
func (c *ChromaV2Client) initializeCollection(ctx context.Context) error {
	// Try to get the collection first
	log.Println("Start initializeCollection")
	count, err := c.client.CountCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to count collections: %w", err)
	}
	log.Printf("Count collections: %d \n", count)

	collection, err := c.client.GetOrCreateCollection(ctx, c.collectionName, v2.WithEmbeddingFunctionCreate(c.embeddings))
	if err != nil {
		return fmt.Errorf("failed  get or create collection: %w", err)
	}

	c.collection = collection
	return nil
}

// UpsertDocument adds or updates a document in ChromaDB via the Go client.
func (c *ChromaV2Client) UpsertDocument(ctx context.Context, entityID string, text string, metadata map[string]interface{}) error {
	// Convert metadata to ChromaDB metadata format
	docMetadata, err := v2.NewDocumentMetadataFromMap(metadata)
	if err != nil {
		// If conversion fails, create empty metadata
		docMetadata = v2.NewDocumentMetadata()
	}

	// Prepare document IDs, texts and metadatas as slices
	docIDs := []v2.DocumentID{v2.DocumentID(entityID)}
	docTexts := []string{text}
	docMetadatas := []v2.DocumentMetadata{docMetadata}

	// Perform upsert operation using options
	err = c.collection.Upsert(ctx,
		v2.WithIDs(docIDs...),
		v2.WithTexts(docTexts...),
		v2.WithMetadatas(docMetadatas...),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert document: %w", err)
	}

	return nil
}

// GetDocuments retrieves documents by their IDs via the Go client.
func (c *ChromaV2Client) GetDocuments(ctx context.Context, entityIDs []string) (map[string]string, error) {
	// Convert string slice to DocumentID slice
	docIDs := make([]v2.DocumentID, len(entityIDs))
	for i, id := range entityIDs {
		docIDs[i] = v2.DocumentID(id)
	}

	// Create get options
	//getOp, err := v2.NewCollectionGetOp(
	//	v2.WithIDsGet(docIDs...),
	//	v2.WithIncludeGet(v2.IncludeDocuments, v2.IncludeMetadatas),
	//)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to create get operation: %w", err)
	//}

	// Get documents from collection
	result, err := c.collection.Get(ctx,
		v2.WithIDsGet(docIDs...),
		v2.WithIncludeGet(v2.IncludeDocuments, v2.IncludeMetadatas))
	if err != nil {
		return nil, fmt.Errorf("failed to get documents: %w", err)
	}

	// Convert results to map
	documents := make(map[string]string)
	idList := result.GetIDs()
	docList := result.GetDocuments()

	for i, id := range idList {
		if i < len(docList) {
			// Convert Document interface to string
			//docStr, ok := docList[i].(string)
			//if !ok {
			//	// If conversion fails, use fmt.Sprintf as fallback
			//	docStr = fmt.Sprintf("%v", docList[i])
			//}
			documents[string(id)] = docList[i].ContentString()
		}
	}

	return documents, nil
}

// SearchEventsByType searches for events by type in ChromaDB via the Go client.
func (c *ChromaV2Client) SearchEventsByType(ctx context.Context, eventType string, limit int) ([]string, error) {
	// Create a where filter to search for documents with specific event type
	whereFilter := v2.EqString("event_type", eventType)

	// Query the collection with the filter
	result, err := c.collection.Query(ctx,
		v2.WithWhereQuery(whereFilter),
		v2.WithNResults(limit),
		v2.WithIncludeQuery(v2.IncludeDocuments, v2.IncludeMetadatas))
	if err != nil {
		return nil, fmt.Errorf("failed to search events by type: %w", err)
	}

	// Extract documents from the result
	var docList = result.GetDocumentsGroups()
	if docList == nil {
		return []string{}, nil
	}

	// Convert documents to string slice
	var documents []string
	for _, doc := range docList[0] {
		//docStr, ok := doc.(string)
		//if !ok {
		//	// If conversion fails, use fmt.Sprintf as fallback
		//	docStr = fmt.Sprintf("%v", doc)
		//}
		documents = append(documents, doc.ContentString())
	}

	return documents, nil
}

// Close closes the ChromaDB client connection.
func (c *ChromaV2Client) Close() error {
	// The client doesn't typically require closing, but we could add cleanup here if needed
	return nil
}
