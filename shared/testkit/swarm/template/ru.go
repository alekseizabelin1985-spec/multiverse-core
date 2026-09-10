// Package template holds the Russian texts the narrator stub writes when
// there is no model to write them (design.md §5, contracts.md C-05).
//
// They are not placeholders. The same texts are what a real role of the swarm
// falls back to when the model times out or answers something the guard
// refuses, so they are written as narrative and kept to the register of the
// dark forest — a text a player sees must read like a text a player sees, even
// when it came from a table (FR-034, generated_by=template).
//
// There are ten of them, the ceiling T-018 sets, and they carry exactly the
// substitutions the stub can honestly make: the name of the character, the
// health the stub knows from the facts of State, the place, and the sequence
// number of a round.
package template

import (
	"hash/fnv"
	"strconv"
	"strings"
)

// Fields are the values a template may substitute. An empty string and a
// negative number mean the stub does not know the value and render as the dash
// the texts read naturally with, so that a gap is visible instead of silently
// swallowed. Zero is a value, not a gap: a character at 0 hp is a character the
// text has something to say about.
//
// Unknown is the negative number to pass for a number that is not known.
const Unknown = -1

type Fields struct {
	// Name is the character the text is about; Names is the list a round text
	// addresses.
	Name  string
	Names string
	// HP and HPMax are what the narrator knows of the character from the facts
	// of State (entity.created, entity.updated); Unknown when it knows
	// nothing.
	HP    int
	HPMax int
	// Place is the region the text happens in.
	Place string
	// Round is the sequence number of a closed round.
	Round int
}

// entries are the texts of a narrative.output kind=entry: one character
// arriving somewhere or looking around.
var entries = []string{
	"{name} останавливается на краю прогалины. Туман глушит шаги; {place} молчит. Сил ещё {hp} из {hp_max}.",
	"Ельник смыкается за спиной. {name} оглядывается: между стволами ничего не движется. Здоровье — {hp}/{hp_max}.",
	"{place}: тропа теряется в подлеске. {name} идёт медленнее, чем хотелось бы, и бережёт оставшиеся {hp} из {hp_max}.",
	"Где-то далеко, за деревьями, воет зверь. {name} стоит неподвижно, пока звук не гаснет. {hp} из {hp_max} — пока держится.",
	"Мох пружинит под ногой. {name} осматривает {place} и не находит ни следов, ни костровища. Сил {hp} из {hp_max}.",
	"Свет сюда почти не доходит. {name} ждёт, пока глаза привыкнут к темноте; здоровья {hp} из {hp_max}.",
}

// rounds are the texts of a narrative.output kind=round: a round of a group
// that has just closed.
//
// They name no health, and that is deliberate: one round is one text for
// everyone in the scope, and there is no single number to put in it. Whose
// health it would be is a question the contract does not answer, so the stub
// does not invent one (C-05, "один narrative.output на раунд группы").
var rounds = []string{
	"Раунд {round} окончен. {names} переводят дух; {place} снова затихает.",
	"Круг {round} завершается. Тишина возвращается быстрее, чем {names} успевают опомниться.",
	"На счёт {round} всё кончается. {names} остаются стоять там, где стояли.",
	"Раунд {round}: {names} сделали то, что успели. Туман сдвигается и закрывает след.",
}

// Entry renders the text of an entry narrative. seed picks which of the texts
// is used and must be a value that is the same on every replay of the same
// journal — the identifier of the event being told about (NFR-061).
func Entry(seed string, f Fields) string { return render(entries, seed, f) }

// Round renders the text of a round narrative.
func Round(seed string, f Fields) string { return render(rounds, seed, f) }

// Count is how many texts the table holds. It exists so that the ceiling of
// T-018 is asserted against the table rather than against a number written
// down twice.
func Count() int { return len(entries) + len(rounds) }

func render(texts []string, seed string, f Fields) string {
	return replacer(f).Replace(texts[pick(seed, len(texts))])
}

// pick chooses a text by the seed. The hash is FNV-1a: a stable function of
// the bytes, unlike a map iteration or a random source, so the same journal
// replays to the same texts.
func pick(seed string, n int) int {
	if n <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return int(h.Sum32() % uint32(n)) //nolint:gosec // n is the length of a table of ten
}

func replacer(f Fields) *strings.Replacer {
	return strings.NewReplacer(
		"{name}", dash(f.Name),
		"{names}", dash(f.Names),
		"{place}", dash(f.Place),
		"{hp}", number(f.HP),
		"{hp_max}", number(f.HPMax),
		"{round}", number(f.Round),
	)
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func number(n int) string {
	if n < 0 {
		return "—"
	}
	return strconv.Itoa(n)
}
