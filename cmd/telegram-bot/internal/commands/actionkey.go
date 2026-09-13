package commands

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// ActionKeyLen is the length of action_key in hex characters.
const ActionKeyLen = 32

// ActionKey is the idempotency key of the action an update became:
// hex(HMAC-SHA256(salt, update_id))[:32] (component §10.3, ADR-018).
//
// The same update handled twice — a repeat after a crash before Telegram saw
// the offset move — yields the same key, so the gateway answers the repeat
// with the first result. The key does not reveal the update id: without the
// salt, which is a secret or a one-way derivative of the token, it cannot be
// inverted or predicted.
func ActionKey(salt []byte, updateID int64) string {
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(strconv.FormatInt(updateID, 10)))
	return hex.EncodeToString(mac.Sum(nil))[:ActionKeyLen]
}
