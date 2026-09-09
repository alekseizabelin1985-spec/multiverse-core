package main

import (
	"context"
	"fmt"
	"os"

	//v2 "github.com/amikos-tech/chroma-go/pkg/api/v2"
	"github.com/amikos-tech/chroma-go/pkg/embeddings/ollama"
	// НЕ импортируем "github.com/amikos-tech/chroma-go/pkg/embeddings/default_ef"
)

func main() {
	url := os.Getenv("CHROMA_URL")
	if url == "" {
		url = "http://chromadb:8000"
	}

	urlEmbeding := os.Getenv("EMBEDING_URL")
	if urlEmbeding == "" {
		urlEmbeding = "http://localhost:11434"
	}

	modelEmbedding := os.Getenv("EMBEDING_MODEL")
	if modelEmbedding == "" {
		modelEmbedding = "nomic-embed-text:latest"
	}

	//client, err := v2.NewHTTPClient(v2.WithBaseURL(url))
	//if err != nil {
	//	fmt.Errorf("failed to create ChromaDB v2 client: %w", err)
	//}

	collectionName := os.Getenv("CHROMA_COLLECTION_NAME")
	if collectionName == "" {
		collectionName = "world_memory"
	}

	ef, err := ollama.NewOllamaEmbeddingFunction(
		ollama.WithBaseURL(urlEmbeding),
		ollama.WithModel("nomic-embed-text:latest"),
	)
	if err != nil {
		fmt.Errorf("Error creating embedding function: %v", err)
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

	//collection, err := client.GetOrCreateCollection(context.Background(), collectionName, v2.WithEmbeddingFunctionCreate(ef))
	//
	//if err != nil {
	//	fmt.Errorf("failed  get or create collection: %w", err)
	//}
	//fmt.Printf("Collection: %v \n", collection)
}
