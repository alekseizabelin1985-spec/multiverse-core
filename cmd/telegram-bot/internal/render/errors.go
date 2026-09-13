package render

import (
	"fmt"
	"time"

	"multiverse-core.io/internal/gateway/api"
)

// hints complete the message of the gateway by the code of the error
// (component §10.5). The gateway speaks of what went wrong; the hint says what
// the player can do next.
var hints = map[string]string{
	api.CodeInEncounter:     "Сначала /flee.",
	api.CodeNotInEncounter:  "Вокруг никого; /look.",
	api.CodeNotLeader:       "Регион меняет лидер группы.",
	api.CodeAlreadyActed:    "Дождитесь конца раунда.",
	api.CodeCharacterDead:   "/start — создать нового.",
	api.CodeConsentRequired: "Прочитайте условия и нажмите «" + ConsentButton + "».",
}

// ErrorText is the answer to an error of the gateway: its message and the
// hint of its code. A code without a hint gets the message alone; an empty
// message falls back to the one of the error table. retryAfter is the
// Retry-After of the answer, 0 when there was none.
//
// An answer without a code is not an answer of the contract — a proxy page, an
// empty 502 — and its message is only the HTTP status text ("Bad Gateway"):
// the player gets Unavailable, as for a network error (component §10.5; Mi-4
// of review #1 of T-311).
//
// Three codes replace the message. rate_limited and the unavailable
// dependencies say the same as the gateway, only in the words of §10.5.
// forget_incomplete must not repeat the message of the gateway at all: it
// speaks of a deletion, and the player would read "deleted" while the external
// id is still in the file (C-08 v1.5, I-2 of T-318).
func ErrorText(code, message string, retryAfter time.Duration) string {
	switch code {
	case "":
		return Unavailable
	case api.CodeForgetIncomplete:
		return ForgetIncomplete(retryAfter)
	case api.CodeBusUnavailable, api.CodeStateUnavailable:
		return Unavailable
	case api.CodeRateLimited:
		if secs := wholeSeconds(retryAfter); secs > 0 {
			return fmt.Sprintf("Слишком часто, подождите %d с.", secs)
		}
		return "Слишком часто, подождите."
	}
	if message == "" {
		if spec, ok := api.LookupError(code); ok {
			message = spec.Message
		} else {
			message = "Не получилось выполнить команду."
		}
	}
	if hint, ok := hints[code]; ok {
		return message + " " + hint
	}
	return message
}
