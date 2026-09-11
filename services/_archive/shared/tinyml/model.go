// internal/tinyml/model.go
package tinyml

import (
	"fmt"
	"sync"
	"time"
)

// TinyModel представляет собой облегченную нейронную модель для Entity-Actor
// Поддерживает до 5000 параметров для эффективного inference
type TinyModel struct {
	mu           sync.RWMutex
	version      string
	weights      map[string]float32
	biases       map[string]float32
	architecture Architecture
	inputSize    int
	outputSize   int
	createdAt    time.Time
	lastUsed     time.Time
}

// Architecture определяет архитектуру нейронной сети
type Architecture struct {
	Layers       []int   // Количество нейронов в каждом слое
	ActivationFn string  // "relu", "sigmoid", "tanh"
	DropoutRate  float32 // 0.0-1.0
}

// ModelConfig конфигурация для создания модели
type ModelConfig struct {
	Version      string
	InputSize    int
	OutputSize   int
	HiddenLayers []int
	ActivationFn string
}

// NewTinyModel создает новую TinyML модель с заданной архитектурой
func NewTinyModel(config ModelConfig) (*TinyModel, error) {
	if config.InputSize <= 0 {
		return nil, fmt.Errorf("input size must be positive")
	}
	if config.OutputSize <= 0 {
		return nil, fmt.Errorf("output size must be positive")
	}

	version := config.Version
	if version == "" {
		version = "v1.0"
	}

	activation := config.ActivationFn
	if activation == "" {
		activation = "relu"
	}

	// Строим архитектуру: input -> hidden layers -> output
	layers := append([]int{config.InputSize}, config.HiddenLayers...)
	layers = append(layers, config.OutputSize)

	model := &TinyModel{
		version:    version,
		weights:    make(map[string]float32),
		biases:     make(map[string]float32),
		inputSize:  config.InputSize,
		outputSize: config.OutputSize,
		architecture: Architecture{
			Layers:       layers,
			ActivationFn: activation,
			DropoutRate:  0.0,
		},
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}

	// Инициализируем веса (Xavier initialization)
	model.initializeWeights()

	return model, nil
}

// initializeWeights инициализирует веса модели
func (m *TinyModel) initializeWeights() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Для простоты инициализируем случайными весами
	// В production здесь будет загрузка из ONNX файла
	for i := 0; i < len(m.architecture.Layers)-1; i++ {
		currentLayer := m.architecture.Layers[i]
		nextLayer := m.architecture.Layers[i+1]

		for n := 0; n < nextLayer; n++ {
			for w := 0; w < currentLayer; w++ {
				key := fmt.Sprintf("w_%d_%d_%d", i, n, w)
				// Xavier initialization
				m.weights[key] = float32(0.0) // В production: random с правильным распределением
			}
			key := fmt.Sprintf("b_%d_%d", i, n)
			m.biases[key] = float32(0.0)
		}
	}
}

// Run выполняет inference модели
// Возвращает map[string]float32 с результатами
func (m *TinyModel) Run(features []float32) (map[string]float32, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(features) != m.inputSize {
		return nil, fmt.Errorf("expected %d features, got %d", m.inputSize, len(features))
	}

	// Обновляем lastUsed
	m.mu.RUnlock()
	m.mu.Lock()
	m.lastUsed = time.Now()
	m.mu.Unlock()
	m.mu.RLock()

	// Forward pass через сеть
	current := make([]float32, len(features))
	copy(current, features)

	for layer := 0; layer < len(m.architecture.Layers)-1; layer++ {
		next := make([]float32, m.architecture.Layers[layer+1])
		_neurons := m.architecture.Layers[layer+1]

		for n := 0; n < _neurons; n++ {
			sum := m.biases[fmt.Sprintf("b_%d_%d", layer, n)]
			for w := 0; w < len(current); w++ {
				weightKey := fmt.Sprintf("w_%d_%d_%d", layer, n, w)
				sum += current[w] * m.weights[weightKey]
			}
			next[n] = m.applyActivation(sum, m.architecture.ActivationFn)
		}
		current = next
	}

	// Преобразуем результат в map
	result := make(map[string]float32, m.outputSize)
	for i := 0; i < m.outputSize; i++ {
		result[fmt.Sprintf("output_%d", i)] = current[i]
	}

	return result, nil
}

// applyActivation применяет функцию активации
func (m *TinyModel) applyActivation(x float32, fn string) float32 {
	switch fn {
	case "relu":
		if x < 0 {
			return 0
		}
		return x
	case "sigmoid":
		return 1.0 / (1.0 + float32(pow(float64(-x), 1)))
	case "tanh":
		return float32(tanh(float64(x)))
	default:
		return x // linear
	}
}

// GetVersion возвращает версию модели
func (m *TinyModel) GetVersion() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.version
}

// GetArchitecture возвращает архитектуру модели
func (m *TinyModel) GetArchitecture() Architecture {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.architecture
}

// GetStats возвращает статистику использования модели
func (m *TinyModel) GetStats() ModelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalParams := len(m.weights) + len(m.biases)

	return ModelStats{
		Version:      m.version,
		TotalParams:  totalParams,
		InputSize:    m.inputSize,
		OutputSize:   m.outputSize,
		CreatedAt:    m.createdAt,
		LastUsed:     m.lastUsed,
		Architecture: m.architecture,
	}
}

// ModelStats статистика модели
type ModelStats struct {
	Version      string
	TotalParams  int
	InputSize    int
	OutputSize   int
	CreatedAt    time.Time
	LastUsed     time.Time
	Architecture Architecture
}

// UpdateWeights обновляет веса модели (для обучения)
func (m *TinyModel) UpdateWeights(weights map[string]float32, biases map[string]float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for k, v := range weights {
		if _, exists := m.weights[k]; !exists {
			return fmt.Errorf("unknown weight key: %s", k)
		}
		m.weights[k] = v
	}

	for k, v := range biases {
		if _, exists := m.biases[k]; !exists {
			return fmt.Errorf("unknown bias key: %s", k)
		}
		m.biases[k] = v
	}

	return nil
}

// Clone создает копию модели
func (m *TinyModel) Clone() *TinyModel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := &TinyModel{
		version:      m.version,
		weights:      make(map[string]float32, len(m.weights)),
		biases:       make(map[string]float32, len(m.biases)),
		inputSize:    m.inputSize,
		outputSize:   m.outputSize,
		architecture: m.architecture,
		createdAt:    m.createdAt,
		lastUsed:     m.lastUsed,
	}

	for k, v := range m.weights {
		clone.weights[k] = v
	}
	for k, v := range m.biases {
		clone.biases[k] = v
	}

	return clone
}

// Вспомогательные математические функции
func pow(x float64, y float64) float64 {
	result := 1.0
	for i := 0; i < int(y); i++ {
		result *= x
	}
	return result
}

func tanh(x float64) float64 {
	exp2x := pow(2.718281828, 2*x)
	return (exp2x - 1) / (exp2x + 1)
}
