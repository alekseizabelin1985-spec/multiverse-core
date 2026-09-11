package mechanics

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"unicode"
)

// The grammar of a rules file, in full (state-and-mechanics.md §5.3):
//
//	check := dice ( '+' term )* '>=' term ( '+' term )*
//	dice  := [INT] 'd' INT [ ('+'|'-') INT ]
//	term  := IDENT | INT
//
// That is the whole language, and the narrowness is the point. A general
// expression engine — the one the as-is shared/rules carries, which parses
// strings like "environment == 'intimate'" at run time — buys flexibility
// nobody asked for and pays for it with float arithmetic, with a surface for
// anything a blueprint feels like writing, and with errors that surface in the
// middle of a fight instead of at Load. Two shapes cover every rule of v0.1;
// widening the grammar is a task with a review of its own (ADR-012 p. 2).

// The identifiers a formula may read off an actor. The left side of a check
// resolves them against the attacker and the right side against the target.
const (
	IdentAtk   = "atk"
	IdentDef   = "def"
	IdentHP    = "hp"
	IdentHPMax = "hp_max"
	IdentFlee  = "flee"
)

// IdentLivingEnemies is the one identifier that comes from neither actor: how
// many enemies are still standing when someone tries to walk away.
const IdentLivingEnemies = "living_enemies"

// actorIdents are legal on both sides of a check; contextIdents only on the
// right, where the threshold is built.
var (
	actorIdents   = []string{IdentAtk, IdentDef, IdentHP, IdentHPMax, IdentFlee}
	contextIdents = []string{IdentLivingEnemies}
)

// The ends of what a formula may ask for. There is no rule in a dark forest
// that needs a thousand dice of a thousand faces, and past that point the
// numbers stop describing a fight: 200000000d6 loads without a word today and
// then spends 466 ms inside a single turn, 10^12 dice spend minutes, and around
// 10^18 the range of the expression no longer fits an int at all.
//
// A formula is not always written by a person under review — Rules.Roll is the
// entry point for background tables of a region GM, whose expressions arrive
// from blueprints (ADR-012 p. 2). So the bound is part of the grammar, checked
// at Load with everything else, rather than a limit the caller is trusted to
// keep. A thousand of each leaves every real formula untouched, costs
// microseconds at the very worst, and holds the result of any expression under
// 10^6 + 10^3, where the arithmetic of a turn cannot overflow.
const (
	maxDiceCount    = 1000
	maxDiceSides    = 1000
	maxDiceModifier = 1000
	// maxTermConst bounds the constant addend of a check for the same reason
	// and by the same amount. Without it "d20 >= 9223372036854775807 + def"
	// loaded in silence and played as a check that always passes: the sum
	// wrapped to a negative threshold. A rule that reads impossible has to be
	// refused where every other complaint is raised — at Load.
	maxTermConst = 1000
)

// DiceExpr is NdM+K: how many dice, how many sides, and a flat modifier. The
// zero value is not a rollable expression; ParseDice makes the only valid ones.
type DiceExpr struct {
	Count    int
	Sides    int
	Modifier int
}

// String renders the expression back into the form a rules file writes, which
// is what goes into Roll.Formula and from there into dice.rolled.
func (d DiceExpr) String() string {
	var b strings.Builder
	if d.Count != 1 {
		b.WriteString(strconv.Itoa(d.Count))
	}
	b.WriteByte('d')
	b.WriteString(strconv.Itoa(d.Sides))
	switch {
	case d.Modifier > 0:
		b.WriteByte('+')
		b.WriteString(strconv.Itoa(d.Modifier))
	case d.Modifier < 0:
		b.WriteString(strconv.Itoa(d.Modifier))
	}
	return b.String()
}

// Min and Max are the ends of the range the expression can produce. Load uses
// Min to refuse a damage formula that could heal the target.
func (d DiceExpr) Min() int { return d.Count + d.Modifier }
func (d DiceExpr) Max() int { return d.Count*d.Sides + d.Modifier }

// Roll throws the expression on one generator: Count draws of 1+IntN(Sides),
// in order, plus the modifier. Natural is the first die alone — the number a
// critical and a fumble are read from, and the number a player sees.
//
// The order matters and is part of the contract: two dice drawn from the same
// generator in the other order give a different sum, and a replay that
// disagrees with the recording about a fight is not a replay (§5.5).
func (d DiceExpr) Roll(rng *rand.Rand) (result, natural int) {
	for i := range d.Count {
		die := 1 + rng.IntN(d.Sides)
		if i == 0 {
			natural = die
		}
		result += die
	}
	return result + d.Modifier, natural
}

// ParseDice reads NdM+K. The count defaults to one, the modifier to none, and
// a die needs at least two sides to be a die at all. Every part is bounded at
// both ends: an expression that parses is one a turn can afford to roll.
func ParseDice(s string) (DiceExpr, error) {
	text := strings.TrimSpace(s)
	if text == "" {
		return DiceExpr{}, fmt.Errorf("empty dice expression")
	}
	if strings.ContainsFunc(text, unicode.IsSpace) {
		return DiceExpr{}, fmt.Errorf("dice expression %q has a space in it", text)
	}

	body, modifier := text, 0
	if i := strings.LastIndexAny(text, "+-"); i > 0 {
		n, err := parseIntStrict(text[i+1:])
		if err != nil {
			return DiceExpr{}, fmt.Errorf("dice modifier %q: %w", text[i:], err)
		}
		if text[i] == '-' {
			n = -n
		}
		body, modifier = text[:i], n
	}

	left, right, found := strings.Cut(body, "d")
	if !found {
		return DiceExpr{}, fmt.Errorf("dice expression %q has no d", text)
	}
	count := 1
	if left != "" {
		n, err := parseIntStrict(left)
		if err != nil {
			return DiceExpr{}, fmt.Errorf("dice count %q: %w", left, err)
		}
		count = n
	}
	sides, err := parseIntStrict(right)
	if err != nil {
		return DiceExpr{}, fmt.Errorf("dice sides %q: %w", right, err)
	}
	switch {
	case count < 1:
		return DiceExpr{}, fmt.Errorf("dice expression %q: at least one die", text)
	case count > maxDiceCount:
		return DiceExpr{}, fmt.Errorf("dice expression %q: %d dice, at most %d: a roll has to finish inside a turn", text, count, maxDiceCount)
	case sides < 2:
		return DiceExpr{}, fmt.Errorf("dice expression %q: at least two sides", text)
	case sides > maxDiceSides:
		return DiceExpr{}, fmt.Errorf("dice expression %q: %d sides, at most %d", text, sides, maxDiceSides)
	case modifier > maxDiceModifier || modifier < -maxDiceModifier:
		return DiceExpr{}, fmt.Errorf("dice expression %q: modifier %d, at most %d either way", text, modifier, maxDiceModifier)
	}
	return DiceExpr{Count: count, Sides: sides, Modifier: modifier}, nil
}

// Term is one addend of a check: either an identifier resolved against an actor
// or the context, or a constant.
type Term struct {
	Ident string
	Const int
}

// String renders the term as a rules file writes it.
func (t Term) String() string {
	if t.Ident != "" {
		return t.Ident
	}
	return strconv.Itoa(t.Const)
}

// CheckExpr is a roll against a threshold: the dice and the bonuses of the
// actor doing the checking on one side, the number to beat on the other.
type CheckExpr struct {
	Dice  DiceExpr
	Left  []Term // added to the roll; resolved against the actor
	Right []Term // the threshold; resolved against the target and the context
}

// String renders the check back into the form a rules file writes.
func (c CheckExpr) String() string {
	parts := []string{c.Dice.String()}
	for _, t := range c.Left {
		parts = append(parts, "+", t.String())
	}
	parts = append(parts, ">=")
	for i, t := range c.Right {
		if i > 0 {
			parts = append(parts, "+")
		}
		parts = append(parts, t.String())
	}
	return strings.Join(parts, " ")
}

// ParseCheck reads the whole grammar. Every error it returns names the formula,
// because the caller that sees it is reading a rules file and not a stack.
func ParseCheck(s string) (CheckExpr, error) {
	left, right, found := strings.Cut(s, ">=")
	if !found {
		return CheckExpr{}, fmt.Errorf("check %q has no >=", s)
	}

	leftParts, err := splitPlus(left)
	if err != nil {
		return CheckExpr{}, fmt.Errorf("check %q: left side: %w", s, err)
	}
	dice, err := ParseDice(leftParts[0])
	if err != nil {
		return CheckExpr{}, fmt.Errorf("check %q: %w", s, err)
	}
	out := CheckExpr{Dice: dice}
	for _, part := range leftParts[1:] {
		term, err := parseTerm(part)
		if err != nil {
			return CheckExpr{}, fmt.Errorf("check %q: left side: %w", s, err)
		}
		out.Left = append(out.Left, term)
	}

	rightParts, err := splitPlus(right)
	if err != nil {
		return CheckExpr{}, fmt.Errorf("check %q: right side: %w", s, err)
	}
	for _, part := range rightParts {
		term, err := parseTerm(part)
		if err != nil {
			return CheckExpr{}, fmt.Errorf("check %q: right side: %w", s, err)
		}
		out.Right = append(out.Right, term)
	}
	return out, nil
}

// CheckResult is one evaluated check: what the dice said, what the sides added
// up to, and whether the roll made it.
type CheckResult struct {
	Natural   int // the first die, before any bonus
	Roll      int // the dice expression alone, modifier included
	Total     int // the roll plus the left-hand terms
	Threshold int // the right-hand terms
	OK        bool
}

// Eval rolls the check. The left-hand identifiers resolve against the actor
// doing the checking, the right-hand ones against the target first and the
// context second — so a rule may compare a bonus of the attacker against a
// number of the defender without either side naming the other.
//
// An identifier nothing answers for is an error rather than a zero: a flight
// bonus that silently reads as zero turns a rule into a coin flip, and the
// caller has no way to notice.
func (c CheckExpr) Eval(rng *rand.Rand, actor, target Actor, ctx map[string]int) (CheckResult, error) {
	roll, natural := c.Dice.Roll(rng)
	out := CheckResult{Natural: natural, Roll: roll, Total: roll}

	for _, t := range c.Left {
		v, err := resolve(t, func(name string) (int, bool) { return actor.Attr(name) })
		if err != nil {
			return CheckResult{}, fmt.Errorf("check %s: actor %s: %w", c, actor.ID, err)
		}
		out.Total += v
	}
	for _, t := range c.Right {
		v, err := resolve(t, func(name string) (int, bool) {
			if v, ok := target.Attr(name); ok {
				return v, true
			}
			v, ok := ctx[name]
			return v, ok
		})
		if err != nil {
			return CheckResult{}, fmt.Errorf("check %s: target %s: %w", c, target.ID, err)
		}
		out.Threshold += v
	}

	out.OK = out.Total >= out.Threshold
	return out, nil
}

// Threshold is the right-hand side alone, without rolling anything. It is what
// combat.decided reports for a flight attempt and what a caller shows a player
// before asking whether they really want to run.
func (c CheckExpr) Threshold(target Actor, ctx map[string]int) (int, error) {
	total := 0
	for _, t := range c.Right {
		v, err := resolve(t, func(name string) (int, bool) {
			if v, ok := target.Attr(name); ok {
				return v, true
			}
			v, ok := ctx[name]
			return v, ok
		})
		if err != nil {
			return 0, fmt.Errorf("check %s: %w", c, err)
		}
		total += v
	}
	return total, nil
}

func resolve(t Term, lookup func(string) (int, bool)) (int, error) {
	if t.Ident == "" {
		return t.Const, nil
	}
	v, ok := lookup(t.Ident)
	if !ok {
		return 0, fmt.Errorf("unknown identifier %q", t.Ident)
	}
	return v, nil
}

// idents reports the identifiers of each side, for the validation Load runs:
// the grammar is checked when the file is read, not when a fight starts.
func (c CheckExpr) idents() (left, right []string) {
	for _, t := range c.Left {
		if t.Ident != "" {
			left = append(left, t.Ident)
		}
	}
	for _, t := range c.Right {
		if t.Ident != "" {
			right = append(right, t.Ident)
		}
	}
	return left, right
}

func parseTerm(s string) (Term, error) {
	text := strings.TrimSpace(s)
	if text == "" {
		return Term{}, fmt.Errorf("empty term")
	}
	if n, err := parseIntStrict(text); err == nil {
		if n > maxTermConst || n < -maxTermConst {
			return Term{}, fmt.Errorf("term %q: constant %d, at most %d either way", text, n, maxTermConst)
		}
		return Term{Const: n}, nil
	}
	if !isIdent(text) {
		return Term{}, fmt.Errorf("term %q is neither an identifier nor a number", text)
	}
	return Term{Ident: text}, nil
}

// splitPlus cuts one side of a check into its addends. An empty addend — a
// trailing plus, a missing threshold — is a malformed formula rather than a
// term worth nothing: silently dropping it would turn "d20 + >= def" into a
// rule that reads correctly and plays differently.
func splitPlus(s string) ([]string, error) {
	parts := strings.Split(s, "+")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty term")
		}
		out = append(out, part)
	}
	return out, nil
}

func isIdent(s string) bool {
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r == '_' && i > 0:
		default:
			return false
		}
	}
	return s != ""
}

func parseIntStrict(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

// parseInt is the forgiving reader Actor.Attr needs: a flee bonus is written
// "+2" as often as "2", and both mean the same thing.
func parseInt(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(s), "+"))
	return n, err == nil
}
