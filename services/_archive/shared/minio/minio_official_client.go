// internal/minio/minio_official_client.go
//
// MinIO Client на основе официальной библиотеки github.com/minio/minio-go/v7
// Использует официальную библиотеку MinIO для работы с хранилищем

package minio

import (
	"context"
	"fmt"
	"io"
	"time"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOOfficialClient — клиент MinIO на основе официальной библиотеки github.com/minio/minio-go/v7
type MinIOOfficialClient struct {
	client *minio.Client
	config Config
}

// NewMinIOOfficialClient создаёт новый MinIO клиент с использованием официальной библиотеки
func NewMinIOOfficialClient(cfg Config) (*MinIOOfficialClient, error) {
	// Создаем клиент MinIO с использованием официальной библиотеки
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return &MinIOOfficialClient{
		client: client,
		config: cfg,
	}, nil
}

// ensureBucket создаёт бакет, если он не существует.
func (c *MinIOOfficialClient) ensureBucket(bucket string) error {
	// Проверяем существование бакета
	exists, err := c.client.BucketExists(context.Background(), bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		// Создаем бакет
		err = c.client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{
			Region: c.config.Region,
		})
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
		}
	}

	return nil
}

// PutObject загружает объект в MinIO.
func (c *MinIOOfficialClient) PutObject(bucket, object string, data io.Reader, size int64) error {
	if err := c.ensureBucket(bucket); err != nil {
		return fmt.Errorf("ensure bucket: %w", err)
	}

	_, err := c.client.PutObject(context.Background(), bucket, object, data, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("put object failed: %w", err)
	}
	return nil
}

// GetObject скачивает объект из MinIO.
func (c *MinIOOfficialClient) GetObject(bucket, object string) ([]byte, error) {
	reader, err := c.client.GetObject(context.Background(), bucket, object, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object failed: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read object data: %w", err)
	}

	return data, nil
}

// ListObjects возвращает список объектов с префиксом.
func (c *MinIOOfficialClient) ListObjects(bucket, prefix string) ([]ObjectInfo, error) {
	if err := c.ensureBucket(bucket); err != nil {
		return nil, err
	}

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Получаем список объектов
	objectCh := c.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var objects []ObjectInfo
	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("list objects failed: %w", object.Err)
		}
		objects = append(objects, ObjectInfo{
			Key:          object.Key,
			LastModified: object.LastModified,
			Size:         object.Size,
		})
	}

	// Сортируем по времени (новые — первыми)
	// Сортировка в обратном порядке (новые первыми)
	for i, j := 0, len(objects)-1; i < j; i, j = i+1, j-1 {
		objects[i], objects[j] = objects[j], objects[i]
	}

	return objects, nil
}

// PresignedGetObject генерирует подписанную ссылку для скачивания.
func (c *MinIOOfficialClient) PresignedGetObject(bucket, object string, expires time.Duration) (string, error) {
	url, err := c.client.PresignedGetObject(context.Background(), bucket, object, expires, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return url.String(), nil
}