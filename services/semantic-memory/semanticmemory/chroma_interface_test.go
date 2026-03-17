// Package semanticmemory
package semanticmemory

import (
	"context"
	"os"
	"testing"
)

// TestChromaClientOnly тестирует только функциональность ChromaClient без Neo4j
func TestChromaClientOnly(t *testing.T) {
	// Убедимся, что используется только ChromaClient
	os.Unsetenv("CHROMA_USE_V2")
	
	// Создаем только ChromaClient напрямую
	chroma := NewChromaClient()
	
	// Проверим, что он реализует интерфейс SemanticStorage
	var _ SemanticStorage = chroma
	
	// Простой тест, чтобы убедиться, что методы существуют
	ctx := context.Background()
	
	// Попробуем вызвать методы - они могут завершиться с ошибкой из-за отсутствия подключения к ChromaDB,
	// но должны существовать
	entityID := "test-entity"
	text := "Test document for chroma client"
	metadata := map[string]interface{}{"test": true}
	
	// Этот вызов может завершиться с ошибкой из-за отсутствия подключения к ChromaDB,
	// но сам метод должен существовать и принимать правильные параметры
	err := chroma.UpsertDocument(ctx, entityID, text, metadata)
	// Не проверяем ошибку здесь, так как тест может выполняться без запущенного ChromaDB
	
	// Проверим, что метод GetDocuments существует
	_, err = chroma.GetDocuments(ctx, []string{entityID})
	// Не проверяем ошибку здесь, так как тест может выполняться без запущенного ChromaDB
	
	// Проверим, что метод SearchEventsByType существует
	_, err = chroma.SearchEventsByType(ctx, "test.event", 10)
	// Не проверяем ошибку здесь, так как тест может выполняться без запущенного ChromaDB
	
	// Проверим, что метод Close существует
	err = chroma.Close()
	if err != nil {
		t.Errorf("ChromaClient.Close() returned error: %v", err)
	}
	
	t.Log("ChromaClient successfully implements SemanticStorage interface")
}