// Package commands parses what a player typed into a Command of the
// dictionary of FR-002 (component §10.3): the game commands enter, leave,
// look, attack, flee, say, rest, defend, the read request status, the group
// commands and the service commands /start, /help and /forget.
//
// A game command is recognised with or without the leading slash, but without
// it only in its exact shape — "look", "attack wolf-alpha" — so that ordinary
// chat such as "leave me alone" is not mistaken for a move; the onboarding
// flow decides what an unrecognised text is. The service commands exist only
// with the slash: /forget deletes the account, and a word in a sentence must
// never start that.
package commands

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Type is the kind of a command. The game types carry the names of the action
// dictionary of the gateway (api-contracts.md §1.4).
type Type string

// The dictionary of FR-002.
const (
	Unknown Type = "unknown"

	Start  Type = "start"
	Help   Type = "help"
	Forget Type = "forget"

	Enter  Type = "enter"
	Leave  Type = "leave"
	Look   Type = "look"
	Attack Type = "attack"
	Flee   Type = "flee"
	Say    Type = "say"
	Rest   Type = "rest"
	Defend Type = "defend"
	Status Type = "status"

	GroupCreate Type = "group.create"
	GroupJoin   Type = "group.join"
	GroupLeave  Type = "group.leave"
)

// MaxSayRunes is the longest say, counted in characters after trimming
// (NFR-049, SEC-11); the gateway refuses a longer one with text_invalid.
const MaxSayRunes = 500

// ForgetConfirmWord is what confirms /forget.
const ForgetConfirmWord = "confirm"

// Problem says why a recognised command cannot be sent as it is.
type Problem string

const (
	// NoProblem: the command is complete.
	NoProblem Problem = ""
	// TextRequired: say without text.
	TextRequired Problem = "text_required"
	// TextTooLong: say longer than MaxSayRunes.
	TextTooLong Problem = "text_too_long"
	// TargetRequired: group join without the id of the group.
	TargetRequired Problem = "target_required"
	// GroupSubcommand: /group without create, join or leave.
	GroupSubcommand Problem = "group_subcommand"
)

// Command is one parsed message.
type Command struct {
	Type Type
	// Target is the region of enter, the NPC of attack or the group of group
	// join; empty when not given.
	Target string
	// Text is what say says. For an Unknown command it is the whole trimmed
	// message, which the flow may still treat as speech.
	Text string
	// Confirm is set on "/forget confirm".
	Confirm bool
	// Slash says the message started with "/".
	Slash bool
	// Problem is set when the command was recognised but cannot be sent;
	// Hint then tells the player what to change.
	Problem Problem
	Hint    string
}

// Parse reads one message.
func Parse(text string) Command {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Command{Type: Unknown}
	}
	slash := strings.HasPrefix(trimmed, "/")
	word, rest := splitWord(strings.TrimPrefix(trimmed, "/"))
	word = strings.ToLower(word)
	if slash {
		// /look@multiverse_bot is how Telegram writes a command picked from
		// the menu of a bot.
		if at := strings.IndexByte(word, '@'); at >= 0 {
			word = word[:at]
		}
	}
	unknown := Command{Type: Unknown, Text: trimmed, Slash: slash}

	switch word {
	case "start", "help":
		if !slash {
			return unknown
		}
		return Command{Type: Type(word), Slash: true}
	case "forget":
		if !slash {
			return unknown
		}
		first, _ := splitWord(rest)
		return Command{Type: Forget, Slash: true, Confirm: strings.EqualFold(first, ForgetConfirmWord)}
	case "say":
		return say(rest, slash)
	case "look", "leave", "flee", "rest", "defend", "status":
		if !slash && rest != "" {
			return unknown
		}
		return Command{Type: Type(word), Slash: slash}
	case "enter", "attack":
		target, extra := splitWord(rest)
		if !slash && extra != "" {
			return unknown
		}
		return Command{Type: Type(word), Target: target, Slash: slash}
	case "group":
		return group(rest, slash, unknown)
	default:
		return unknown
	}
}

func say(rest string, slash bool) Command {
	cmd := Command{Type: Say, Text: rest, Slash: slash}
	switch n := utf8.RuneCountInString(rest); {
	case n == 0:
		cmd.Problem, cmd.Hint = TextRequired, "Напишите, что сказать: /say <текст>."
	case n > MaxSayRunes:
		cmd.Problem = TextTooLong
		cmd.Hint = fmt.Sprintf("Слишком длинная реплика: %d символов, можно не больше %d. Сократите и отправьте снова.", n, MaxSayRunes)
	}
	return cmd
}

func group(rest string, slash bool, unknown Command) Command {
	sub, tail := splitWord(rest)
	target, extra := splitWord(tail)
	switch strings.ToLower(sub) {
	case "create", "leave":
		if tail != "" && !slash {
			return unknown
		}
		t := GroupCreate
		if strings.EqualFold(sub, "leave") {
			t = GroupLeave
		}
		return Command{Type: t, Slash: slash}
	case "join":
		if (target == "" || extra != "") && !slash {
			return unknown
		}
		cmd := Command{Type: GroupJoin, Target: target, Slash: slash}
		if target == "" {
			cmd.Problem, cmd.Hint = TargetRequired, "Укажите номер группы: /group join <id>."
		}
		return cmd
	default:
		if !slash {
			return unknown
		}
		return Command{Type: Unknown, Text: unknown.Text, Slash: true, Problem: GroupSubcommand,
			Hint: "Команды группы: /group create, /group join <id>, /group leave."}
	}
}

// splitWord returns the first word of s and the trimmed rest.
func splitWord(s string) (word, rest string) {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	end := strings.IndexFunc(s, unicode.IsSpace)
	if end < 0 {
		return s, ""
	}
	return s[:end], strings.TrimSpace(s[end:])
}
