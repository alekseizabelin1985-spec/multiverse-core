// internal/minio/factory.go
//
// Фабрика для создания клиентов MinIO с возможностью выбора реализации

package minio

import (
	"fmt"
)

// ClientType определяет тип клиента MinIO
type ClientType int

const (
	// HTTPClientType - клиент с ручной реализацией AWS Signature V4
	HTTPClientType ClientType = iota
	// MinIOClientType - клиент с использованием официальной библиотеки minio-go
	MinIOClientType
)

// NewClientFromType создает новый клиент MinIO в зависимости от типа
func NewClientFromType(cfg Config, clientType ClientType) (ClientInterface, error) {
	switch clientType {
	case HTTPClientType:
		return NewClientHTTP(cfg)
	case MinIOClientType:
		return NewMinIOOfficialClient(cfg)
	default:
		return nil, fmt.Errorf("unknown client type: %d", clientType)
	}
}

// NewClientHTTP создает клиент с ручной реализацией AWS Signature V4
func NewClientHTTP(cfg Config) (*Client, error) {
	return newClientHTTP(cfg)
}