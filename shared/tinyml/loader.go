// internal/tinyml/loader.go
package tinyml

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"multiverse-core/internal/minio"
)

// ModelLoader загрузчик моделей из хранилища
type ModelLoader struct {
	minioClient *minio.MinIOOfficialClient
	bucketName  string
	cache       map[string]*TinyModel // Simple in-memory cache
}

// NewModelLoader создает загрузчик моделей
func NewModelLoader(minioClient *minio.MinIOOfficialClient, bucketName string) *ModelLoader {
	return &ModelLoader{
		minioClient: minioClient,
		bucketName:  bucketName,
		cache:       make(map[string]*TinyModel),
	}
}

// ModelMetadata метаданные модели
type ModelMetadata struct {
	Version      string             `json:"version"`
	Architecture Architecture       `json:"architecture"`
	Weights      map[string]float32 `json:"weights"`
	Biases       map[string]float32 `json:"biases"`
	InputSize    int                `json:"input_size"`
	OutputSize   int                `json:"output_size"`
	CreatedAt    time.Time          `json:"created_at"`
	EntityType   string             `json:"entity_type"`
}

// LoadFromStorage загружает модель из MinIO
func (l *ModelLoader) LoadFromStorage(modelID string) (*TinyModel, error) {
	// Проверяем кэш
	if cached, ok := l.cache[modelID]; ok {
		return cached, nil
	}

	if l.minioClient == nil {
		return nil, fmt.Errorf("minio client not initialized")
	}

	// Загружаем из MinIO
	objectKey := fmt.Sprintf("models/%s.json", modelID)
	data, err := l.minioClient.GetObject(l.bucketName, objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get model from storage: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	// Парсим JSON
	var metadata ModelMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal model metadata: %w", err)
	}

	// Создаем модель
	model := &TinyModel{
		version:      metadata.Version,
		weights:      metadata.Weights,
		biases:       metadata.Biases,
		inputSize:    metadata.InputSize,
		outputSize:   metadata.OutputSize,
		architecture: metadata.Architecture,
		createdAt:    metadata.CreatedAt,
		lastUsed:     time.Now(),
	}

	// Кэшируем
	l.cache[modelID] = model

	return model, nil
}

// SaveToStorage сохраняет модель в MinIO
func (l *ModelLoader) SaveToStorage(modelID string, model *TinyModel) error {
	if l.minioClient == nil {
		return fmt.Errorf("minio client not initialized")
	}

	model.mu.RLock()
	defer model.mu.RUnlock()

	metadata := ModelMetadata{
		Version:      model.version,
		Architecture: model.architecture,
		Weights:      model.weights,
		Biases:       model.biases,
		InputSize:    model.inputSize,
		OutputSize:   model.outputSize,
		CreatedAt:    model.createdAt,
		EntityType:   "generic", // Можно передать как параметр
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal model metadata: %w", err)
	}

	objectKey := fmt.Sprintf("models/%s.json", modelID)
	if err := l.minioClient.PutObject(l.bucketName, objectKey, strings.NewReader(string(data)), int64(len(data))); err != nil {
		return fmt.Errorf("failed to put model to storage: %w", err)
	}

	// Обновляем кэш
	l.cache[modelID] = model

	return nil
}

// ClearCache очищает кэш моделей
func (l *ModelLoader) ClearCache() {
	l.cache = make(map[string]*TinyModel)
}

// RemoveFromCache удаляет модель из кэша
func (l *ModelLoader) RemoveFromCache(modelID string) {
	delete(l.cache, modelID)
}

// GetCacheStats возвращает статистику кэша
func (l *ModelLoader) GetCacheStats() CacheStats {
	totalParams := 0
	for _, model := range l.cache {
		stats := model.GetStats()
		totalParams += stats.TotalParams
	}

	return CacheStats{
		CachedModels: len(l.cache),
		TotalParams:  totalParams,
	}
}

// CacheStats статистика кэша моделей
type CacheStats struct {
	CachedModels int `json:"cached_models"`
	TotalParams  int `json:"total_params"`
}

// ExportToONNX экспортирует модель в ONNX формат (заглушка для future implementation)
func (m *TinyModel) ExportToONNX() ([]byte, error) {
	// В production здесь будет реальная экспортирование в ONNX
	return json.Marshal(map[string]interface{}{
		"version":      m.version,
		"architecture": m.architecture,
		"weights":      m.weights,
		"biases":       m.biases,
	})
}

// ImportFromONNX импортирует модель из ONNX формата (заглушка для future implementation)
func ImportFromONNX(data []byte) (*TinyModel, error) {
	// В production здесь будет парсинг ONNX файла
	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	// Создаем заглушку модели
	return NewTinyModel(ModelConfig{
		Version:      "onnx-imported",
		InputSize:    10,
		OutputSize:   5,
		HiddenLayers: []int{8},
	})
}
