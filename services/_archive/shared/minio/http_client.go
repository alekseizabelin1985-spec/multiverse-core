// internal/minio/http_client.go
//
// MinIO HTTP Client — полная реализация AWS Signature V4.
// Использует только стандартную библиотеку Go.

package minio

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Client — HTTP-клиент для MinIO.
type Client struct {
	config  Config
	http    *http.Client
	baseURL *url.URL
}

// newClientHTTP создаёт новый MinIO HTTP-клиент.
func newClientHTTP(cfg Config) (*Client, error) {
	// 🔑 Добавляем схему, если её нет
	endpoint := cfg.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if cfg.UseSSL {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}

	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	return &Client{
		config: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}, nil
}

// ensureBucket создаёт бакет, если он не существует.
func (c *Client) ensureBucket(bucket string) error {
	req, err := c.newRequest("HEAD", bucket, "", nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil // Бакет существует
	}
	if resp.StatusCode == 404 {
		// Создаём бакет
		createReq, err := c.newRequest("PUT", bucket, "", nil)
		if err != nil {
			return err
		}
		createResp, err := c.http.Do(createReq)
		if err != nil {
			return err
		}
		defer createResp.Body.Close()
		if createResp.StatusCode >= 400 {
			body, _ := io.ReadAll(createResp.Body)
			return fmt.Errorf("failed to create bucket %s: %d %s", bucket, createResp.StatusCode, string(body))
		}
		return nil
	}
	return fmt.Errorf("unexpected status for HEAD bucket: %d", resp.StatusCode)
}

// PutObject загружает объект в MinIO.
func (c *Client) PutObject(bucket, object string, data io.Reader, size int64) error {
	if err := c.ensureBucket(bucket); err != nil {
		return fmt.Errorf("ensure bucket: %w", err)
	}

	// Читаем всё в память (для подписи)
	body, err := io.ReadAll(data)
	if err != nil {
		return err
	}

	req, err := c.newRequest("PUT", bucket, object, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(body))

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("put object failed: %d %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetObject скачивает объект из MinIO.
func (c *Client) GetObject(bucket, object string) ([]byte, error) {
	req, err := c.newRequest("GET", bucket, object, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("object not found: %s/%s", bucket, object)
	}
	if resp.StatusCode >= 400 {

		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get object failed: %d %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// ListObjects возвращает список объектов с префиксом.
func (c *Client) ListObjects(bucket, prefix string) ([]ObjectInfo, error) {
	if err := c.ensureBucket(bucket); err != nil {
		return nil, err
	}

	// Создаем URL с параметрами до создания запроса
	u := *c.baseURL
	if bucket != "" {
		u.Path = "/" + bucket
	}

	params := url.Values{}
	if prefix != "" {
		params.Set("prefix", prefix)
	}
	params.Set("delimiter", "")
	params.Set("list-type", "2")
	u.RawQuery = params.Encode()

	fmt.Println(u.String())
	// Создаем новый запрос с правильным URL, содержащим параметры
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	// Устанавливаем обязательные заголовки
	date := time.Now().UTC().Format("20060102T150405Z")

	// 🔑 КРИТИЧЕСКОЕ ИСПРАВЛЕНИЕ: используем ПОЛНЫЙ хост (с портом) для подписи
	host := u.Host
	req.Header.Set("Host", host)

	req.Header.Set("x-amz-date", date)

	// Для GET-запросов без тела используем хэш пустой строки
	h := sha256.Sum256([]byte{})
	hashedPayload := hex.EncodeToString(h[:])
	req.Header.Set("x-amz-content-sha256", hashedPayload)

	// Подписываем запрос (только если есть ключи)
	if c.config.AccessKeyID != "" && c.config.SecretAccessKey != "" {
		signature := c.signRequest(req, date)
		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=%s",
			c.config.AccessKeyID,
			date[:8], // YYYYMMDD
			c.config.Region,
			signature)
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list objects failed: %d %s", resp.StatusCode, string(body))
	}

	var result ListBucketResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var objects []ObjectInfo
	for _, c := range result.Contents {
		objects = append(objects, ObjectInfo{
			Key:          c.Key,
			LastModified: c.LastModified,
			Size:         c.Size,
		})
	}
	// Сортируем по времени (новые — первями)
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(objects[j].LastModified)
	})
	return objects, nil
}

// PresignedGetObject генерирует подписанную ссылку для скачивания.
func (c *Client) PresignedGetObject(bucket, object string, expires time.Duration) (string, error) {
	return fmt.Sprintf("%s/%s/%s?debug=1", c.baseURL.String(), bucket, object), nil
}

// --- Внутренние вспомогательные методы ---

// newRequest создаёт подписанной HTTP-запрос к MinIO.
func (c *Client) newRequest(method, bucket, object string, body io.Reader) (*http.Request, error) {
	u := *c.baseURL
	if bucket != "" {
		u.Path = "/" + bucket
		if object != "" {
			u.Path += "/" + object
		}
	}

	// Подготовим тело запроса для подписи и последующего использования
	var bodyBytes []byte
	var hashedPayload string

	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		h := sha256.Sum256(bodyBytes)
		hashedPayload = hex.EncodeToString(h[:])
	} else {
		h := sha256.Sum256([]byte{})
		hashedPayload = hex.EncodeToString(h[:])
	}

	req, err := http.NewRequest(method, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	// Устанавливаем обязательные заголовки
	date := time.Now().UTC().Format("20060102T150405Z")

	// 🔑 КРИТИЧЕСКОЕ ИСПРАВЛЕНИЕ: используем ПОЛНЫЙ хост (с портом) для подписи
	// В подписи должен использоваться тот же хост, что и в запросе
	// Если в запросе "minio:9090", то и в заголовке "Host: minio:9090"
	host := u.Host
	req.Header.Set("Host", host)

	req.Header.Set("x-amz-date", date)
	req.Header.Set("x-amz-content-sha256", hashedPayload)

	// Подписываем запрос (только если есть ключи)
	if c.config.AccessKeyID != "" && c.config.SecretAccessKey != "" {
		signature := c.signRequest(req, date)
		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=%s",
			c.config.AccessKeyID,
			date[:8], // YYYYMMDD
			c.config.Region,
			signature)
		req.Header.Set("Authorization", authHeader)
	}

	return req, nil
}

// signRequest вычисляет подпись запроса (AWS Signature V4).
func (c *Client) signRequest(req *http.Request, date string) string {
	// 1. Canonical Request
	canonicalRequest := c.buildCanonicalRequest(req)

	// 2. String to Sign
	stringToSign := c.buildStringToSign(canonicalRequest, date)

	// 3. Signature
	signature := c.calculateSignature(stringToSign, date[:8])

	return signature
}

func (c *Client) buildCanonicalRequest(req *http.Request) string {
	method := req.Method
	uri := req.URL.Path
	if uri == "" {
		uri = "/"
	}

	// ВАЖНО: строка запроса должна быть правильно отформатирована
	// в соответствии со спецификацией AWS Signature Version 4
	query := c.canonicalQueryString(req.URL.Query())

	// Canonical Headers
	// 🔑 КРИТИЧЕСКОЕ ИСПРАВЛЕНИЕ: сортируем заголовки в алфавитном порядке
	// и используем нижний регистр для ключей
	var headers []string
	for k := range req.Header {
		headers = append(headers, strings.ToLower(k))
	}
	sort.Strings(headers)

	var canonicalHeaders strings.Builder
	for _, k := range headers {
		// 🔑 КРИТИЧЕСКОЕ ИСПРАВЛЕНИЕ: значения заголовков должны быть
		// с удалёнными лишними пробелами
		value := strings.TrimSpace(req.Header.Get(k))
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", k, value))
	}

	signedHeaders := strings.Join(headers, ";")

	// Hashed Payload - используем значение из заголовка x-amz-content-sha256
	hashedPayload := req.Header.Get("x-amz-content-sha256")

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		uri,
		query,
		canonicalHeaders.String(),
		signedHeaders,
		hashedPayload)
}

// canonicalQueryString формирует каноническую строку запроса в соответствии с AWS спецификацией
func (c *Client) canonicalQueryString(queryValues url.Values) string {
	if len(queryValues) == 0 {
		return ""
	}

	var keys []string
	for k := range queryValues {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		// Значения для одного ключа должны быть отсортированы
		values := queryValues[k]
		sort.Strings(values)
		for _, v := range values {
			// Кодируем ключ и значение согласно спецификации
			encodedK := c.uriEncode(k, false)
			encodedV := c.uriEncode(v, false)
			pairs = append(pairs, fmt.Sprintf("%s=%s", encodedK, encodedV))
		}
	}

	return strings.Join(pairs, "&")
}

// uriEncode кодирует строку в соответствии с AWS спецификацией
func (c *Client) uriEncode(str string, encodeSlash bool) string {
	var encoded strings.Builder
	for i := 0; i < len(str); i++ {
		c := str[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '~' || c == '.' {
			encoded.WriteByte(c)
		} else if c == '/' && !encodeSlash {
			encoded.WriteByte(c)
		} else {
			encoded.WriteString(fmt.Sprintf("%%%.2X", c))
		}
	}
	return encoded.String()
}

func (c *Client) buildStringToSign(canonicalRequest, date string) string {
	hash := sha256.Sum256([]byte(canonicalRequest))
	return fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s/%s/s3/aws4_request\n%x",
		date,
		date[:8],
		c.config.Region,
		hash)
}

func (c *Client) calculateSignature(stringToSign, date string) string {
	// 1. Derive signing key
	kDate := hmacSHA256([]byte("AWS4"+c.config.SecretAccessKey), date)
	kRegion := hmacSHA256(kDate, c.config.Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")

	// 2. Signature
	signature := hmacSHA256(kSigning, stringToSign)
	return hex.EncodeToString(signature)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// --- Структуры для XML-парсинга ---

type ListBucketResult struct {
	Contents []Content `xml:"Contents"`
}

type Content struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	Size         int64     `xml:"Size"`
}
