// Package template holds the Russian texts the narrator stub writes when
// there is no model to write them (design.md §5, contracts.md C-05).
//
// They are not placeholders. The same texts are what a real role of the swarm
// falls back to when the model times out or answers something the guard
// refuses (T-229), so they are written as narrative and kept to the register of
// the dark forest — a text a player sees must read like a text a player sees,
// even when it came from a table (FR-034, generated_by=template).
//
// There is one table per kind of narrative.output C-05 names, and for the turn
// one per way an exchange can end. They carry exactly the substitutions the
// stub can honestly make: the name of the character, the health the stub knows,
// the place, the sequence number of a round, whom the character fights and the
// damage the exchange dealt both ways.
//
// A text carries no markup a messenger would read as formatting: no angle
// brackets, no asterisks, no underscores, no backticks (SEC-10) — the bot sends
// it as plain text, and a text that relied on a parse_mode would either show
// its markup or be refused. The placeholders obey the same rule, so that the
// table can be checked as it is written.
package template

import (
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
)

// Unknown is the negative number to pass for a number that is not known.
const Unknown = -1

// Fields are the values a template may substitute. An empty string and a
// negative number mean the stub does not know the value and render as the dash
// the texts read naturally with, so that a gap is visible instead of silently
// swallowed. Zero is a value, not a gap: a character at 0 hp is a character the
// text has something to say about.
type Fields struct {
	// Name is the character the text is about; Names is the list a round or an
	// encounter text addresses.
	Name  string
	Names string
	// HP and HPMax are what the narrator knows of the character; Unknown when it
	// knows nothing.
	HP    int
	HPMax int
	// Place is the region the text happens in.
	Place string
	// Round is the sequence number of a closed round.
	Round int
	// Foe is whom the character fights: the NPC of an exchange, or the NPCs of
	// an encounter that has just opened.
	Foe string
	// Dealt is the damage the character's blow did, and Taken the damage the
	// character took in the answer. Both are zero when nothing landed.
	Dealt int
	Taken int
}

// Outcome is how one exchange of a fight ended, as the turn text tells it.
type Outcome int

// The ways an exchange can end. They are the branches the decisions of C-03
// can take in one turn and nothing more: a blow lands or misses, a blow kills,
// a flight succeeds, is caught, or fails with nothing striking.
const (
	// Hit: the character's blow landed and the foe is still standing.
	Hit Outcome = iota
	// Miss: the character's blow found nothing.
	Miss
	// Kill: the character's blow finished the foe.
	Kill
	// Fled: the character got away.
	Fled
	// Caught: the flight failed and the foe struck out of turn.
	Caught
	// Stuck: the flight failed and nothing struck — the rules call for no
	// strike, or nobody is left standing to deal one.
	Stuck

	outcomes
)

// Outcomes are all the ways an exchange can end, for a caller that has to walk
// them — a test, above all, that has to reach every text of the turn table.
func Outcomes() []Outcome { return []Outcome{Hit, Miss, Kill, Fled, Caught, Stuck} }

// entries are the texts of a narrative.output kind=entry: one character
// arriving somewhere or looking around.
var entries = []string{
	"{name} останавливается на краю прогалины. Туман глушит шаги; {place} молчит. Сил ещё {hp} из {hpmax}.",
	"Ельник смыкается за спиной. {name} оглядывается: между стволами ничего не движется. Здоровье — {hp}/{hpmax}.",
	"{place}: тропа теряется в подлеске. {name} идёт медленнее, чем хотелось бы, и бережёт оставшиеся {hp} из {hpmax}.",
	"Где-то далеко, за деревьями, воет зверь. {name} стоит неподвижно, пока звук не гаснет. {hp} из {hpmax} — пока держится.",
	"Мох пружинит под ногой. {name} осматривает {place} и не находит ни следов, ни костровища. Сил {hp} из {hpmax}.",
	"Свет сюда почти не доходит. {name} ждёт, пока глаза привыкнут к темноте; здоровья {hp} из {hpmax}.",
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

// turns are the texts of a narrative.output kind=turn, one table per way the
// exchange ended. The answer of the foe is written as a number and not as a
// verb, because a bite that missed is an answer of zero and a text that said
// "the wolf bites" over it would be telling the player something that did not
// happen. For the same reason a flight nobody struck at has a table of its own
// (Stuck) that names no blow and no foe: the foe may be lying dead.
var turns = [outcomes][]string{
	Hit: {
		"{name} достаёт {foe}: удар на {dealt}. В ответ — {taken}. Сил {hp} из {hpmax}.",
		"Клинок находит цель: {foe} теряет {dealt}. Ответный выпад — {taken}. {name}: {hp} из {hpmax}.",
	},
	Miss: {
		"{name} бьёт мимо, {foe} отвечает: {taken}. Сил {hp} из {hpmax}.",
		"Удар уходит в пустоту. {foe} отвечает — {taken}; {name} держится: {hp} из {hpmax}.",
	},
	Kill: {
		"{name} наносит последний удар — {foe} падает в мох и больше не встаёт.",
		"Удар на {dealt}, и {foe} затихает. {name} переводит дух: {hp} из {hpmax}.",
	},
	Fled: {
		"{name} вырывается из схватки и уходит в туман. {foe} остаётся позади.",
	},
	Caught: {
		"{name} пытается уйти, но {foe} отрезает путь и бьёт вдогонку: {taken}. Сил {hp} из {hpmax}.",
	},
	Stuck: {
		"{name} пытается уйти, но тропа петляет и выводит обратно, туда же. Уйти не вышло; сил {hp} из {hpmax}.",
	},
}

// deaths are the texts of a narrative.output kind=death: the character of the
// player has died. Only a death — a character who was forgotten by their player
// is not told about at all (C-02 v1.2).
var deaths = []string{
	"{name} падает и больше не поднимается. Лес затихает, будто ничего не было.",
	"Для этого пути всё кончено: {name} лежит во мху, и туман медленно смыкается над тропой.",
}

// worldEvents are the texts of a narrative.output kind=world_event: the world
// did something to the characters, and in Phase 1 the one thing it does is
// open an encounter.
var worldEvents = []string{
	"Из подлеска выходит {foe}. Отступать поздно — схватка начинается: {names}.",
	"Туман расходится, и {foe} оказывается совсем рядом. {place}: здесь и сейчас начинается бой, и в нём {names}.",
}

// Entry renders the text of an entry narrative. seed picks which of the texts
// is used and must be a value that is the same on every replay of the same
// journal — the identifier of the event being told about (NFR-061).
func Entry(seed string, f Fields) string { return render(entries, seed, f) }

// Round renders the text of a round narrative.
func Round(seed string, f Fields) string { return render(rounds, seed, f) }

// Turn renders the text of a turn narrative for an exchange that ended the way
// o says.
func Turn(seed string, o Outcome, f Fields) string { return render(turns[o], seed, f) }

// Death renders the text of a death narrative.
func Death(seed string, f Fields) string { return render(deaths, seed, f) }

// WorldEvent renders the text of a world_event narrative.
func WorldEvent(seed string, f Fields) string { return render(worldEvents, seed, f) }

// Texts is every text of every table, as it is written — placeholders and all.
// It exists so that what the tables promise (SEC-10, reachability, the size of
// the table) is checked against the tables themselves rather than against
// what a few seeds happen to render.
func Texts() []string {
	out := slices.Concat(entries, rounds, deaths, worldEvents)
	for _, table := range turns {
		out = append(out, table...)
	}
	return out
}

// Count is how many texts the tables hold.
func Count() int { return len(Texts()) }

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
	return int(h.Sum32() % uint32(n)) //nolint:gosec // n is the length of a table of a few texts
}

func replacer(f Fields) *strings.Replacer {
	return strings.NewReplacer(
		"{name}", dash(f.Name),
		"{names}", dash(f.Names),
		"{place}", dash(f.Place),
		"{foe}", dash(f.Foe),
		"{hp}", number(f.HP),
		"{hpmax}", number(f.HPMax),
		"{round}", number(f.Round),
		"{dealt}", number(f.Dealt),
		"{taken}", number(f.Taken),
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
