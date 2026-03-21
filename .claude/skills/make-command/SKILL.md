---
name: make-command
description: Выполнение команд Makefile для multiverse-core (build, test, logs, run)
---

Этот навык выполняет стандартные Makefile команды для multiverse-core.

## Доступные команды

### Сборка
- `make build` - собрать все сервисы (Docker)
- `make build-service SERVICE=<name>` - собрать конкретный сервис
- `make build-all` - собрать все сервисы локально

### Запуск
- `make up` - запустить все сервисы (Docker Compose)
- `make run SERVICE=<name>` - запустить конкретный сервис
- `make down` - остановить все сервисы

### Логи
- `make logs` - логи всех сервисов
- `make logs-service SERVICE=<name>` - логи конкретного сервиса

### Тесты
- `make test` - запустить все тесты
- `make test-service SERVICE=<name>` - тесты конкретного сервиса

### Поддерживающие
- `make clean` - очистить артефакты сборки
- `make sync` - синхронизировать Go workspace

## Использование

Вызовите `/make-command` и укажите нужную команду.

## Примеры

```
/build-service SERVICE=narrative-orchestrator
/run SERVICE=world-generator
/logs-service SERVICE=semantic-memory
/test-service SERVICE=entity-manager
```