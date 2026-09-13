package render

import (
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/internal/gateway/api"
)

// Kinds of a delivery (api-contracts.md §1.5).
const (
	KindAck        = "ack"
	KindMechanics  = "mechanics"
	KindNarrative  = "narrative"
	KindWorldEvent = "world_event"
	KindGroup      = "group"
	KindSystem     = "system"
)

// Sources of a text of a delivery.
const (
	GeneratedByRules    = "rules"
	GeneratedByLLM      = "llm"
	GeneratedByTemplate = "template"
)

// Marks of a narrative (component §10.4, NFR-045, FR-052).
const (
	MarkLLM      = "— текст создан ИИ"
	MarkTemplate = "— упрощённый режим (шаблон)"
)

// MaxMessageLen is the longest text of one sendMessage, in the UTF-16 code
// units Telegram counts: an emoji outside the basic plane takes two.
const MaxMessageLen = 4096

// Delivery renders a delivery as the messages to send, in order (component
// §10.4): mechanics, group and ack as they are; a narrative with the mark of
// where its text came from; a world event with the combat keyboard; a system
// message with /start. A text longer than MaxMessageLen is cut into several
// messages by paragraphs, and the keyboard goes with the last one. A delivery
// without text gives no message, a narrative without text included: a mark
// alone says nothing (N-4 of review #1 of T-311).
func Delivery(d api.Delivery) []Reply {
	if strings.TrimSpace(d.Text) == "" {
		return nil
	}
	text := d.Text
	kb := keyboardOf(d.Kind)
	if d.Kind == KindNarrative {
		text = withMark(text, d.GeneratedBy, d.FallbackReason)
	}
	parts := Split(text)
	if len(parts) == 0 {
		return nil
	}
	out := make([]Reply, len(parts))
	for i, p := range parts {
		out[i] = Reply{Text: p}
	}
	out[len(out)-1].Keyboard = kb
	return out
}

func keyboardOf(kind string) *sender.Keyboard {
	switch kind {
	case KindWorldEvent:
		return CombatKeyboard(nil)
	case KindSystem:
		return StartKeyboard()
	default:
		return nil
	}
}

// withMark appends the mark of a narrative as a line of its own. Every
// narrative gets one (NFR-045): a template names itself and its reason, and
// any other source — llm, or a value this bot does not know — is marked as
// written by the AI, since a text wrongly marked as AI misleads nobody about
// its risk and an unmarked AI text does.
func withMark(text, generatedBy string, fallbackReason *string) string {
	mark := MarkLLM
	if generatedBy == GeneratedByTemplate {
		mark = MarkTemplate
		if fallbackReason != nil && strings.TrimSpace(*fallbackReason) != "" {
			mark += ": " + strings.TrimSpace(*fallbackReason)
		}
	}
	return text + "\n" + mark
}

// Split cuts text into messages of at most MaxMessageLen: between paragraphs
// first, then between lines of a paragraph too long for one message, then
// between words of a line too long, and inside a word only when nothing else
// is left. The separators at a cut are dropped; nothing else is changed.
//
// No part is empty or blank: Telegram refuses such a message, and the keyboard
// that goes with the last part would be lost with it (Mi-5 of review #1 of
// T-311). A blank text gives no part at all.
func Split(text string) []string {
	var parts []string
	if len16(text) <= MaxMessageLen {
		parts = []string{text}
	} else {
		parts = pack(text, "\n\n", func(p string) []string {
			return pack(p, "\n", splitLine)
		})
	}
	out := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pack joins the parts of text separated by sep greedily into pieces of at
// most MaxMessageLen; a part too long on its own is cut further by finer.
func pack(text, sep string, finer func(string) []string) []string {
	var (
		out     []string
		current string
		has     bool
	)
	flush := func() {
		if has {
			out = append(out, current)
			current, has = "", false
		}
	}
	for _, part := range strings.Split(text, sep) {
		if len16(part) > MaxMessageLen {
			flush()
			out = append(out, finer(part)...)
			continue
		}
		if has && len16(current)+len16(sep)+len16(part) <= MaxMessageLen {
			current += sep + part
			continue
		}
		flush()
		current, has = part, true
	}
	flush()
	return out
}

// splitLine cuts a line at the last space that fits, or inside a word when a
// word alone is longer than a message.
func splitLine(line string) []string {
	var out []string
	for len16(line) > MaxMessageLen {
		cut, units, lastSpace := 0, 0, -1
		for i, r := range line {
			n := utf16.RuneLen(r)
			if n < 0 {
				n = 1
			}
			if units+n > MaxMessageLen {
				break
			}
			units += n
			cut = i + utf8.RuneLen(r)
			if unicode.IsSpace(r) {
				lastSpace = i
			}
		}
		if lastSpace > 0 {
			out = append(out, line[:lastSpace])
			_, size := utf8.DecodeRuneInString(line[lastSpace:])
			line = line[lastSpace+size:]
			continue
		}
		out = append(out, line[:cut])
		line = line[cut:]
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func len16(s string) int {
	n := 0
	for _, r := range s {
		if l := utf16.RuneLen(r); l > 0 {
			n += l
		} else {
			n++
		}
	}
	return n
}
