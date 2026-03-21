---
name: docker-compose-control
description: Управление docker-compose сервисами multiverse-core
disable-model-invocation: true
---

Управление сервисами Docker Compose для multiverse-core.

## Команды

### Статус
- `/dc-status` - статус всех контейнеров
- `/dc-logs <service>` - логи сервиса

### Запуск/Остановка
- `/dc-start` - запустить все
- `/dc-stop` - остановить все
- `/dc-restart <service>` - перезапустить сервис
- `/dc-pull` - скачать новые образы

### Очистка
- `/dc-down` - полностью остановить
- `/dc-cleanup` - очистить volumes и образы

## Примеры

```
/dc-start
/dc-logs redpanda
/dc-restart entity-manager
/dc-cleanup
```

## Зависимости
- Docker Compose v2.0+
- docker CLI