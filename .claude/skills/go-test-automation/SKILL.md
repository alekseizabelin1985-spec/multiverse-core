---
name: go-test-automation
description: Автоматизация тестирования Go сервисов multiverse-core с анализом покрытия и race detection
disable-model-invocation: true
---

Автоматизирует запуск тестов для Go сервисов с расширенной аналитикой.

## Доступные команды

### Запуск тестов
- `/go-test` - запустить все тесты в workspace
- `/go-test-service SERVICE=<name>` - тесты конкретного сервиса
- `/go-test-unit` - только unit тесты (быстро)
- `/go-test-integration` - интеграционные тесты

### Анализ покрытия
- `/go-coverage` - показать покрытие для всех сервисов
- `/go-coverage-service SERVICE=<name>` - покрытие конкретного сервиса
- `/go-coverage-html` - открыть HTML отчет

### Race Detection
- `/go-race-test` - запустить тесты с детектором гонок
- `/go-race-service SERVICE=<name>` - race detection для сервиса

### Отчеты
- `/go-test-report` - сводка результатов всех тестов
- `/go-slow-tests` - показать самые медленные тесты

## Использование

### Базовый запуск тестов

```bash
/go-test
```

Запустит `go test ./...` во всем workspace.

### Тесты конкретного сервиса

```bash
/go-test-service SERVICE=entity-manager
```

Или для narrative-orchestrator:

```bash
/go-test-service SERVICE=narrative-orchestrator
```

### Интеграционные тесты

```bash
/go-test-integration
```

Запустит тесты с тегом `integration`.

###Race Detection (выявление гонок)

```bash
/go-race-test
```

Запустит все тесты с `-race` флагом.

### Coverage analysis

```bash
/go-coverage
```

Покажет покрытие по всем сервисам.

## Конфигурация

### Переменные окружения

| Variable | Default | Description |
|----------|---------|-------------|
| `GO_TEST_FLAGS` | `-v` | Флаги для go test |
| `GO_TEST_TIMEOUT` | `10m` | Таймаут тестов |
| `COVERAGE_THRESHOLD` | `60%` | Минимальное покрытие |

### Рекомендуемые флаги

```bash
# Unit тесты (быстро)
go test -v -short ./...

# Интеграционные тесты
go test -v -tags=integration ./...

# С race detection
go test -v -race ./...

# С coverage
go test -v -coverprofile=coverage.out ./...
```

## Примеры

### Найти failing тесты

```bash
/go-test-service SERVICE=narrative-orchestrator
# Проверит вывод на наличие FAIL
```

### Проанализировать медленные тесты

```bash
# Добавить флаг для timing
timeout 60s go test -v -count=1 -run Test.* ./services/narrative-orchestrator/...
```

### Проверить coverage перед merge

```bash
/go-coverage-service SERVICE=entity-manager
# Проверит, что покрытие > 60%
```

## Тесты в workspace

### Shared modules
- `shared/eventbus/eventbus_test.go` - Kafka client tests
- `shared/minio/minio_test.go` - MinIO client tests

### Service tests
- `services/narrative-orchestrator/narrativeorchestrator/prompt_builder_test.go`
- `services/semantic-memory/semanticmemory/*_test.go`
- `services/world-generator/worldgenerator/generator_test.go`

### Worktree tests (для каждого worktree)
- Повторяются тесты для всех worktrees (jolly-rhodes, laughing-kare, и т.д.)

## Чек-лист перед коммитом

- [ ] Все unit тесты проходят
- [ ] Race detection не выявляет проблем
- [ ] Coverage не падает ниже порога
- [ ] Интеграционные тесты проходят

## Troubleshooting

### Тесты ждут подключения к Redpanda

```bash
# Запустить тесты с флагом -short для пропуска интеграционных
/go-test-unit
```

### Race condition в event handlers

```bash
# Изолировать проблемный тест
/go-race-service SERVICE=entity-manager -- -run TestSpecificHandler
```

### Low coverage report

```bash
# Открыть HTML отчет для детального анализа
/go-coverage-html
```

## Интеграция с CI/CD

```bash
#!/bin/bash
# .github/workflows/test.yml

- name: Run Go Tests
  run: |
    make build-all
    make test

- name: Race Detection
  run: |
    go test -race -coverprofile=coverage.out ./...

- name: Upload Coverage
  uses: codecov/codecov-action@v3
  with:
    file: ./coverage.out
```

## Ссылки

- Makefile: `make test`, `make test-service`
- Docker Compose: `redpanda-init` для тестирования
- Docs: `shared/eventbus/`, `services/*/README.md`
