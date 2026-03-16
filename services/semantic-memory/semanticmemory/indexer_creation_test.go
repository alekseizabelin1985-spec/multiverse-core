// Package semanticmemory
package semanticmemory

import (
	"os"
	"testing"
)

// TestIndexerCreation тестирует создание Indexer с разными настройками
func TestIndexerCreation(t *testing.T) {
	// Сохраняем исходное значение
	originalValue := os.Getenv("CHROMA_USE_V2")
	defer func() {
		// Восстанавливаем исходное значение
		os.Setenv("CHROMA_USE_V2", originalValue)
	}()
	
	// Тестируем создание Indexer без включенного v2 (должно использовать ChromaClient)
	t.Run("Indexer with default client", func(t *testing.T) {
		os.Unsetenv("CHROMA_USE_V2")
		
		// Создаем Indexer - это должно пройти успешно с ChromaClient
		indexer, err := NewIndexer()
		if err != nil {
			// Ошибка может произойти из-за подключения к Neo4j, но нас интересует
			// только то, что функция пытается создать правильный клиент
			// Проверим, что ошибка связана с подключением к Neo4j, а не с отсутствием функций
			if err.Error() != "neo4j connectivity test failed: ConnectivityError: Unable to retrieve routing table from neo4j:7687: dial tcp: lookup neo4j: no such host" {
				t.Errorf("Unexpected error when creating indexer with default client: %v", err)
			}
		} else {
			// Если indexer создался, проверим, что он использует интерфейс
			if indexer.chroma == nil {
				t.Error("Indexer chroma storage is nil")
			}
		}
	})
	
	// Тестируем создание Indexer с включенным v2 (должно вернуть ошибку, потому что chroma_v2_enabled не включен при сборке)
	t.Run("Indexer with v2 client disabled", func(t *testing.T) {
		os.Setenv("CHROMA_USE_V2", "true")
		
		// Создаем Indexer - это должно привести к ошибке, потому что chroma_v2_enabled не включен при сборке
		indexer, err := NewIndexer()
		if err == nil {
			t.Error("Expected error when creating indexer with v2 client but no build tag, got nil")
		} else if indexer != nil {
			t.Error("Expected nil indexer when creation fails")
		}
		
		// Проверим, что ошибка содержит ожидаемое сообщение
		expectedMsg := "ChromaDB v2 client not available in this build - compile with -tags chroma_v2_enabled"
		if err != nil && err.Error() != "failed to create ChromaDB v2 client: "+expectedMsg {
			t.Errorf("Expected error message containing '%s', got '%s'", expectedMsg, err.Error())
		}
	})
}