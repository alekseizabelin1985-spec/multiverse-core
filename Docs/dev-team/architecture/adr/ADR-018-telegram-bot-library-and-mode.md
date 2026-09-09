# ADR-018: Telegram-бот — библиотека `go-telegram/bot`, long polling с последовательной обработкой, reply-клавиатуры, без собственного хранилища

Статус: предложено (к утверждению на G2 с планом EPIC-004) · Дата: 2026-09-09 · Автор: architect#3 (TEAM-3, EPIC-004) · Уровень: реализация блока
Связи: ADR-006 (long polling, outbox с ack — не пересматривается), ADR-009 п. 4 (бот не использует данные профиля; логи без пейлоадов), C-08; FR-004, FR-009, FR-060, BR-07, BR-09; NFR-003, NFR-041, NFR-092; `integrations.md` §1; `components/gateway-and-bot.md` §10.

## Контекст

ADR-006 зафиксировал транспорт (long polling `getUpdates`, `timeout=25`, `allowed_updates=[message]`, один экземпляр) и оставил архитектору команды выбор библиотеки (кандидат `github.com/go-telegram/bot`). Требования к боту: тонкий клиент без персистентного состояния, внешний ID не покидает бота и `links.db`, логи без пейлоадов, ack игроку ≤ 300 мс, порядок сообщений одного игрока сохраняется, уведомление/18+/согласие с явным подтверждением, `/forget` с подтверждением. Замечание системного архитектора: бот **без собственной таблицы** — внешний ID приходит в `Delivery.route` из хранилища связок.

## Рассмотренные варианты

| Вопрос | Вариант | Плюсы | Минусы |
|---|---|---|---|
| Библиотека | **`github.com/go-telegram/bot` v1.25.0** (2026-09-01, Bot API 10.3, MIT, zero-dependency, Go ≥ 1.18) | активно поддерживается, без транзитивных зависимостей, есть `WithAllowedUpdates`, `WithNotAsyncHandlers`, `WithErrorsHandler`, `WithDefaultHandler`, `StartWebhook` для E-H | нет встроенного FSM диалога (пишем свой, ~150 строк) |
| | `go-telegram-bot-api/telegram-bot-api` v5 | самый известный | обновления Bot API отстают, поддержка нерегулярная |
| | `mymmrac/telego` | полный, генерируемый API | зависимости (fasthttp, sonic), избыточен для 12 команд |
| | свой HTTP-клиент к Bot API | ноль зависимостей | ~500 строк повторяют библиотеку; сериализация типов Bot API |
| Обработка обновлений | **последовательная (`WithNotAsyncHandlers`)** | порядок команд одного игрока; ноль гонок в FSM диалога; ≤ 6 игроков — задержка незаметна | один медленный `sendMessage` тормозит очередь (митигируется: ответ игроку «принято» не ждёт доставок — они идут отдельной горутиной `deliver`) |
| | параллельная с воркерами и мьютексом на чат | масштаб | сложнее, не нужно при MVP-1 |
| Подтверждение согласия | **reply-клавиатура (текстовые кнопки)** | остаётся в `allowed_updates=[message]` (ADR-006); текст кнопки — явное подтверждение, видимое в истории чата | кнопка присылает обычный текст — разбираем по точному совпадению |
| | inline-клавиатура + `callback_query` | компактнее | требует `allowed_updates=[message, callback_query]` — отклонение от ADR-006; callback без текста в истории хуже как доказательство согласия |
| Состояние бота | **только память: FSM диалога на чат (TTL 15 мин), кэш `chat_id → player_id` (TTL 1 ч)** | нет второй копии ПДн; после рестарта `/start` восстанавливает всё через `links/resolve`; `Delivery.route.external_id` = `chat_id` личного чата | потеря шага онбординга при рестарте (игрок повторяет `/start`) |
| | SQLite/файл в боте | переживает рестарт | вторая копия внешних ID вне `links.db` (нарушает ADR-009 п. 2 «единственная копия») |
| `action_key` | **`HMAC-SHA256(salt, update_id)[:32 hex]`**, salt из env (по умолчанию `SHA-256(token)`) | необратим; стабилен при повторной обработке того же обновления; не требует хранилища | смена salt после рестарта меняет ключи — безопасно, т.к. Telegram не переотдаёт подтверждённые обновления |
| Групповая доставка | **только личные чаты в MVP-1** | без хранения `chat_id` группы | «общий чат группы» из ADR-006 п. 5 — отложено (E-H); запрос system-architect в `components/gateway-and-bot.md` §14 п. 8 |

## Решение

1. Библиотека — `github.com/go-telegram/bot` **v1.25.0** (пин в `go.mod`; обновления — по мере выхода Bot API, отдельной задачей). Инициализация: `bot.New(token, bot.WithAllowedUpdates([]string{"message"}), bot.WithNotAsyncHandlers(), bot.WithDefaultHandler(flow.Handle), bot.WithErrorsHandler(privacy.ErrorLog))`; `b.Start(ctx)` с `timeout=25 с`. `WithDebug` в проде запрещён (пишет пейлоады).
2. Интерфейсы `updates.UpdateSource{Start(ctx, func(Update)) error}` и `sender.Sender{Send(ctx, chatID int64, text string, kb *ReplyKeyboard) error}` — единственные точки контакта с библиотекой; тесты используют `FakeUpdateSource`/`FakeSender`; webhook-реализация `UpdateSource` — E-H.
3. Из `Update` читаются только `update_id`, `message.chat.id`, `message.from.id`, `message.text`; `from.username`/`first_name` сравниваются с именем персонажа в памяти и отбрасываются (DR-10), никуда не пишутся. Групповые чаты Telegram отклоняются сообщением «пишите в личные сообщения».
4. Онбординг — FSM на чат в памяти: `idle → awaiting_consent → awaiting_name → awaiting_name_confirm → awaiting_world → ready`; подтверждение согласия — кнопка reply-клавиатуры с текстом «Подтверждаю: мне 18+ и я согласен» (точное совпадение), после чего `POST /v1/links/consent {notice_shown, consent, age_confirmed: true, shown_at}`. Уведомление повторяется по `/help` и при `notice_due=true` из `resolve` (FR-009).
5. Доставки — отдельная горутина `deliver.Loop` (long-poll `wait_ms=25000`, `limit=100`): `sendMessage(chat_id = route.external_id)` → пакетный ack; при сбое Telegram — повтор ≤ 3, затем без ack (gateway повторит через 30 с); `403/400 chat not found` — ack (игрок недоступен) и лог без ID. Бот не переупорядочивает: порядок на игрока гарантирует gateway (одна доставка в лизинге).
6. Один экземпляр: `409 Conflict` от `getUpdates` → лог `handled=true`, выход с кодом 3 (compose `restart: on-failure` не должен бесконечно рестартовать второй экземпляр — DevOps ставит `restart: unless-stopped` только одному сервису бота).
7. Логи — `slog` через `privacy.Handler`: атрибуты `chat_id`, `external_id`, `username`, `text` отбрасываются на любом уровне; уровень `info` по умолчанию; retention ≤ 7 дней — драйвер логов Docker (DevOps, NFR-041).
8. Конфигурация — только env (`TELEGRAM_BOT_TOKEN`, `BOT_GATEWAY_URL`, `BOT_CLIENT_ID`, `BOT_ACTION_KEY_SALT`, `BOT_POLL_TIMEOUT`, `BOT_DELIVERY_WAIT`, `BOT_DIALOG_TTL`, `LOG_LEVEL`); отсутствие токена — ошибка старта с понятным сообщением.

## Последствия

- Позитивные: одна зависимость без транзитивных; ADR-006 соблюдён буквально (`allowed_updates=[message]`); бот не хранит ПДн — тест NFR-041 проверяет только логи бота; детерминированные e2e бота через фейки источника/отправителя.
- Негативные: последовательная обработка ограничивает пропускную способность (~10 обновлений/с) — достаточно для ≤ 6 игроков; при росте — `WithWorkers(N)` + мьютекс на чат без изменения контрактов; рестарт бота сбрасывает незавершённый онбординг.
- Что придётся сделать: EPIC-004 I1 — `cmd/telegram-bot` по `components/gateway-and-bot.md` §10; DevOps — retention логов, `restart`-политика, публикация порта gateway на `127.0.0.1`; system-architect — решение по п. 8 §14 компонентного документа (общий чат группы).

## Дополнение после G2 (2026-09-09, сведение `consolidation.md` §3/§6; статус ADR — принят на G2)

1. **Allowlist и только личные чаты (SEC-06/07, решение пользователя U-7; ADR-006 дополнение п. 1–2)**: перед FSM (п. 4) добавляется компонент `access.Gate` — первый шаг обработки `Update`: `chat.type != private` → отказ; `from.id ∉ MV_TELEGRAM_ALLOWED_USER_IDS` → одно сообщение «доступ по приглашению», gateway не вызывается, ID не логируется, счётчик `bot_denied_total`; пустой список = отказ всем. Связка — только по `from.id`. Общий чат группы (п. «Групповая доставка») — подтверждено как E-H.
2. **Лимит команд (SEC-11)**: 20 команд/мин на user id (`MV_BOT_RATE_COMMANDS_PER_MIN`, token bucket в памяти) — в том же `access.Gate`, до вызова gateway.
3. **Редакция токена (SEC-08; ADR-006 дополнение п. 3)**: п. 7 расширяется — `privacy.Handler` не только отбрасывает атрибуты, но и заменяет подстроку `bot<digits>:<token>` → `bot<redacted>` в тексте любого сообщения лога и ошибки (`WithErrorsHandler` оборачивает ошибки библиотеки через `privacy.Redact`); unit-тест «строка ошибки не содержит токен», включая `409 Conflict`.
4. **Без `parse_mode` (SEC-10; ADR-006 дополнение п. 4)**: `Sender.Send` не имеет параметра разметки; UGC и нарратив — plain text; тест с `[x](http://…)`, `<a>`, `*`.
5. **Env с префиксом `MV_` (G-11/D-4)**: п. 8 читать как `MV_TELEGRAM_BOT_TOKEN`, `MV_TELEGRAM_ALLOWED_USER_IDS`, `MV_BOT_GATEWAY_URL`, `MV_BOT_CLIENT_ID`, `MV_BOT_ACTION_KEY_SALT`, `MV_BOT_RATE_COMMANDS_PER_MIN`, `MV_BOT_POLL_TIMEOUT`, `MV_BOT_DELIVERY_WAIT`, `MV_BOT_DIALOG_TTL`, `MV_LOG_LEVEL`; объявление — `shared/env.Declare`.
6. **Логи бота (D-12)**: ротация по объёму `json-file 10m × 3`, «≤ 7 дней» — ориентир (ADR-006 дополнение п. 5).
