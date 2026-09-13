package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
)

// Error codes of the gateway HTTP API: the table of api-contracts.md §1.6 plus
// the additions of C-08 v1.1–v1.3. A client matches on the code, never on the
// message, so a code is renamed only through a new API version.
const (
	CodeInvalidRequest       = "invalid_request"
	CodeUnknownAction        = "unknown_action"
	CodeTextInvalid          = "text_invalid"
	CodeNameRequired         = "name_required"
	CodeNameInvalid          = "name_invalid"
	CodeConsentIncomplete    = "consent_incomplete"
	CodeConsentRequired      = "consent_required"
	CodeActorKindForbidden   = "actor_kind_forbidden"
	CodeClientUnknown        = "client_unknown"
	CodeClientMismatch       = "client_mismatch"
	CodePlayerNotFound       = "player_not_found"
	CodeWorldNotFound        = "world_not_found"
	CodeUnknownTarget        = "unknown_target"
	CodeCharacterDead        = "character_dead"
	CodeInEncounter          = "in_encounter"
	CodeNotInEncounter       = "not_in_encounter"
	CodeTargetDead           = "target_dead"
	CodeNoLeader             = "no_leader"
	CodeNotLeader            = "not_leader"
	CodeNotInRegion          = "not_in_region"
	CodeAlreadyActed         = "already_acted"
	CodeNoOpenRound          = "no_open_round"
	CodePollInProgress       = "poll_in_progress"
	CodeAlreadyInGroup       = "already_in_group"
	CodeNotInGroup           = "not_in_group"
	CodeGroupFull            = "group_full"
	CodeGroupInEncounter     = "group_in_encounter"
	CodeEncounterUnavailable = "encounter_unavailable"
	CodeCharacterAliveExists = "character_alive_exists"
	CodePayloadTooLarge      = "payload_too_large"
	CodeFilterError          = "filter_error"
	CodeRateLimited          = "rate_limited"
	CodeInternal             = "internal"
	CodeNotImplemented       = "not_implemented"
	CodeBusUnavailable       = "bus_unavailable"
	CodeStateUnavailable     = "state_unavailable"
)

// ErrorSpec is one row of the error table: the code, the HTTP status it always
// travels with and the default message for the player (ru, plain text).
type ErrorSpec struct {
	Code    string
	Status  int
	Message string
}

// errorTable is the single source of the codes: api/gateway.openapi.yaml lists
// the same codes under the same statuses, and openapi_test.go keeps the two in
// step. T-303 and later tasks add rows here and in the spec together.
var errorTable = []ErrorSpec{
	{CodeInvalidRequest, http.StatusBadRequest, "Некорректный запрос."},
	{CodeUnknownAction, http.StatusBadRequest, "Такого действия нет."},
	{CodeTextInvalid, http.StatusBadRequest, "Текст должен быть от 1 до 500 символов."},
	{CodeNameRequired, http.StatusBadRequest, "Нужно имя персонажа."},
	{CodeNameInvalid, http.StatusBadRequest, "Имя: от 2 до 32 символов — буквы, цифры, пробел, дефис."},
	{CodeConsentIncomplete, http.StatusBadRequest, "Нужно подтвердить уведомление, согласие и возраст 18+."},

	{CodeConsentRequired, http.StatusForbidden, "Сначала нужно дать согласие."},
	{CodeActorKindForbidden, http.StatusForbidden, "Этому клиенту такой режим запрещён."},
	{CodeClientUnknown, http.StatusForbidden, "Клиент не допущен."},
	{CodeClientMismatch, http.StatusForbidden, "Клиент в пути не совпадает с заголовком."},

	{CodePlayerNotFound, http.StatusNotFound, "Персонаж не найден."},
	{CodeWorldNotFound, http.StatusNotFound, "Мир не найден."},
	{CodeUnknownTarget, http.StatusNotFound, "Цель не найдена."},

	{CodeCharacterDead, http.StatusConflict, "Персонаж больше не может действовать."},
	{CodeInEncounter, http.StatusConflict, "Сейчас идёт бой."},
	{CodeNotInEncounter, http.StatusConflict, "Сейчас нет боя."},
	{CodeTargetDead, http.StatusConflict, "Цель уже повержена."},
	{CodeNoLeader, http.StatusConflict, "У группы нет лидера."},
	{CodeNotLeader, http.StatusConflict, "Это может сделать только лидер группы."},
	{CodeNotInRegion, http.StatusConflict, "Отсюда некуда уходить."},
	{CodeAlreadyActed, http.StatusConflict, "В этом раунде вы уже действовали."},
	{CodeNoOpenRound, http.StatusConflict, "Открытого раунда нет."},
	{CodePollInProgress, http.StatusConflict, "Запрос доставок уже выполняется."},
	{CodeAlreadyInGroup, http.StatusConflict, "Вы уже в группе."},
	{CodeNotInGroup, http.StatusConflict, "Вы не в группе."},
	{CodeGroupFull, http.StatusConflict, "Группа заполнена."},
	{CodeGroupInEncounter, http.StatusConflict, "Группа в бою."},
	{CodeEncounterUnavailable, http.StatusConflict, "Встреча ещё не готова, повторите чуть позже."},
	{CodeCharacterAliveExists, http.StatusConflict, "Живой персонаж уже есть."},

	{CodePayloadTooLarge, http.StatusRequestEntityTooLarge, "Слишком большой запрос."},
	{CodeFilterError, http.StatusUnprocessableEntity, "Не удалось проверить текст."},
	{CodeRateLimited, http.StatusTooManyRequests, "Слишком много действий, подождите."},

	{CodeInternal, http.StatusInternalServerError, "Внутренняя ошибка."},
	{CodeNotImplemented, http.StatusNotImplemented, "Не реализовано."},
	{CodeBusUnavailable, http.StatusServiceUnavailable, "Сервис временно недоступен, повторите."},
	{CodeStateUnavailable, http.StatusServiceUnavailable, "Сервис временно недоступен, повторите."},
}

var errorIndex = indexErrors(errorTable)

func indexErrors(table []ErrorSpec) map[string]ErrorSpec {
	idx := make(map[string]ErrorSpec, len(table))
	for _, spec := range table {
		idx[spec.Code] = spec
	}
	return idx
}

// ErrorSpecs returns a copy of the error table in declaration order.
func ErrorSpecs() []ErrorSpec { return slices.Clone(errorTable) }

// LookupError returns the row of code.
func LookupError(code string) (ErrorSpec, bool) {
	spec, ok := errorIndex[code]
	return spec, ok
}

// Error is an error response of the gateway: the status and message come from
// the table, the details from the handler. It is the Go error value, not the
// wire type: the schema Error of the spec is ErrorResponse, returned by Body.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

// NewError builds the error of a code from the table. A code outside the table
// is a defect of the caller, not a condition of the request, so it panics; the
// recover middleware turns that into 500 internal (component §5.1).
func NewError(code string, details map[string]any) *Error {
	spec, ok := LookupError(code)
	if !ok {
		panic(fmt.Sprintf("api: error code %q is not in the error table", code))
	}
	return &Error{Status: spec.Status, Code: spec.Code, Message: spec.Message, Details: details}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message)
}

// Body returns the wire form {"error": {...}}.
func (e *Error) Body() ErrorResponse {
	return ErrorResponse{Error: ErrorBody{Code: e.Code, Message: e.Message, Details: e.Details}}
}

// WriteError writes e as the JSON error body. Headers that belong to a code,
// such as Retry-After of rate_limited, are set by the caller before the call.
func WriteError(w http.ResponseWriter, e *Error) error {
	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(e.Status)
	if err := json.NewEncoder(w).Encode(e.Body()); err != nil {
		return fmt.Errorf("api: write error %s: %w", e.Code, err)
	}
	return nil
}
