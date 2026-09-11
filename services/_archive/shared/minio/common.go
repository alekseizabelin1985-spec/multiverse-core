// internal/minio/common.go
//
// Общие определения для клиентов MinIO

package minio

import (
	"io"
	"time"
)

// Config для MinIO-клиента.
type Config struct {
	Endpoint        string // Например: "minio:9090" (без http://)
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	Region          string // По умолчанию "us-east-1"
}

// ObjectInfo информация об объекте
type ObjectInfo struct {
	Key          string
	LastModified time.Time
	Size         int64
}

// ClientInterface определяет общий интерфейс для всех реализаций MinIO клиента
type ClientInterface interface {
	// PutObject загружает объект в MinIO
	PutObject(bucket, object string, data io.Reader, size int64) error
	
	// GetObject скачивает объект из MinIO
	GetObject(bucket, object string) ([]byte, error)
	
	// ListObjects возвращает список объектов с префиксом
	ListObjects(bucket, prefix string) ([]ObjectInfo, error)
	
	// PresignedGetObject генерирует подписанную ссылку для скачивания
	PresignedGetObject(bucket, object string, expires time.Duration) (string, error)
}