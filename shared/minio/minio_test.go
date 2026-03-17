package minio

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestMinioConnection(t *testing.T) {
	// Получаем параметры подключения из переменных окружения
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	endpoint = "127.0.0.1:9000"
	accessKey = "minioadmin"
	secretKey = "minioadmin"

	if endpoint == "" || accessKey == "" || secretKey == "" {
		t.Skip("Skipping test: MinIO credentials not provided")

	}

	cfg := Config{
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		UseSSL:          false,
		Region:          "us-east-1",
	}

	//client, err := NewClient(cfg)
	client, err := NewMinIOOfficialClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create MinIO client: %v", err)
	}

	// Тестирование PutObject
	t.Run("PutObject", func(t *testing.T) {
		data := []byte("test file content")
		key := fmt.Sprintf("test-put-object-%d", time.Now().Unix())

		err := client.PutObject("test-bucket", key, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("Failed to put object: %v", err)
		}
	})

	// Тестирование GetObject
	t.Run("GetObject", func(t *testing.T) {
		data := []byte("test file content for get")
		key := fmt.Sprintf("test-get-object-%d", time.Now().Unix())

		// Сначала загружаем объект
		err := client.PutObject("test-bucket", key, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("Failed to put object: %v", err)
		}

		// Затем получаем его
		result, err := client.GetObject("test-bucket", key)
		if err != nil {
			t.Fatalf("Failed to get object: %v", err)
		}

		if string(result) != string(data) {
			t.Errorf("Expected %s, got %s", string(data), string(result))
		}
	})

	// Тестирование ListObjects
	t.Run("ListObjects", func(t *testing.T) {
		// Создаем несколько объектов для тестирования
		data1 := []byte("test content 1")
		data2 := []byte("test content 2")
		key1 := fmt.Sprintf("test-list-objects-1-%d", time.Now().Unix())
		key2 := fmt.Sprintf("test-list-objects-2-%d", time.Now().Unix())

		// Загружаем объекты
		err := client.PutObject("test-bucket", key1, bytes.NewReader(data1), int64(len(data1)))
		if err != nil {
			t.Fatalf("Failed to put object 1: %v", err)
		}

		err = client.PutObject("test-bucket", key2, bytes.NewReader(data2), int64(len(data2)))
		if err != nil {
			t.Fatalf("Failed to put object 2: %v", err)
		}

		// Получаем список объектов
		objects, err := client.ListObjects("test-bucket", "test-list-objects")
		if err != nil {
			t.Fatalf("Failed to list objects: %v", err)
		}

		if len(objects) < 2 {
			t.Errorf("Expected at least 2 objects, got %d", len(objects))
		}
	})

	// Тестирование ошибок
	t.Run("ErrorHandling", func(t *testing.T) {
		// Попытка получить несуществующий объект
		_, err := client.GetObject("test-bucket", "non-existent-object")
		if err == nil {
			t.Error("Expected error for non-existent object")
		}

		// Попытка получить объект из несуществующего бакета
		_, err = client.GetObject("non-existent-bucket", "any-object")
		if err == nil {
			t.Error("Expected error for non-existent bucket")
		}
	})

	t.Log("Successfully connected to MinIO and performed all tests")
}
