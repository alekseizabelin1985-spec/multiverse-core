# MinIO Clients

Этот пакет предоставляет две реализации клиента MinIO для различных сценариев использования:

## Реализации

### 1. HTTP Client (ручная подпись AWS Signature V4)
Файл: `http_client.go`

- Полностью реализован на стандартной библиотеке Go
- Использует ручную реализацию AWS Signature V4
- Не требует внешних зависимостей помимо стандартной библиотеки
- Подходит для случаев, когда нужно избежать дополнительных зависимостей

### 2. Официальный клиент MinIO
Файл: `minio_official_client.go`

- Использует официальную библиотеку `github.com/minio/minio-go/v7`
- Обеспечивает полную совместимость с MinIO
- Рекомендуется для новых разработок

## Использование

### Простое использование (обратная совместимость)
```go
import "multiverse-core/internal/minio"

// Создание HTTP клиента (ручная подпись) - для обратной совместимости
cfg := minio.Config{
    Endpoint:        "localhost:9000",
    AccessKeyID:     "your-access-key",
    SecretAccessKey: "your-secret-key",
    UseSSL:          false,
}

client, err := minio.NewClient(cfg) // Создает HTTP клиент с ручной подписью
if err != nil {
    log.Fatal(err)
}

// Использование клиента
err = client.PutObject("bucket", "object", reader, size)
```

### Использование фабрики для выбора реализации
```go
import "multiverse-core/internal/minio"

// Создание клиента через фабрику
client, err := minio.NewClientFromType(cfg, minio.HTTPClientType) // или minio.MinIOClientType
if err != nil {
    log.Fatal(err)
}
```

### Использование конкретных конструкторов
```go
import "multiverse-core/internal/minio"

// Создание HTTP клиента напрямую
httpClient, err := minio.NewClientHTTP(cfg)
if err != nil {
    log.Fatal(err)
}

// Создание официального клиента напрямую
officialClient, err := minio.NewMinIOOfficialClient(cfg)
if err != nil {
    log.Fatal(err)
}
```

## Совместимость

Обе реализации реализуют одинаковый интерфейс `ClientInterface` с одинаковыми сигнатурами методов, что позволяет легко переключаться между ними.

## Структуры

### Config
Конфигурация для обоих клиентов:
- `Endpoint` - адрес сервера MinIO (например, "localhost:9000")
- `AccessKeyID` - ID ключа доступа
- `SecretAccessKey` - секретный ключ доступа
- `UseSSL` - использовать ли SSL
- `Region` - регион (по умолчанию "us-east-1")

### Методы

Обе реализации предоставляют следующие методы:
- `PutObject(bucket, object string, data io.Reader, size int64) error` - загрузка объекта
- `GetObject(bucket, object string) ([]byte, error)` - получение объекта
- `ListObjects(bucket, prefix string) ([]ObjectInfo, error)` - список объектов
- `PresignedGetObject(bucket, object string, expires time.Duration) (string, error)` - подписанная ссылка на объект