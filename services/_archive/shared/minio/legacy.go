// internal/minio/legacy.go
//
// Функции для обеспечения обратной совместимости

package minio

// NewClient создаёт новый MinIO HTTP-клиент (для обратной совместимости).
// Использует HTTP-реализацию с ручной подписью.
func NewClient(cfg Config) (*Client, error) {
	return newClientHTTP(cfg)
}