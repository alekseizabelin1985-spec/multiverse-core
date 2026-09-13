package render_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"multiverse-core.io/cmd/telegram-bot/internal/render"
)

// noticeSubstrings are the 24 substrings of tasks/T-318.md §3, lower case.
// Each occurs exactly once in the notice, so that removing a section is not
// hidden by the same words in another one (Mi-6, R2-Mi-2 of T-318).
var noticeSubstrings = []string{
	"искусственный интеллект",
	"подтверждаете свой возраст",
	"только на компьютере оператора",
	"может включить облачную модель",
	"за рубеж",
	"написанные раньше",
	"но без вашего telegram id",
	"хранится ваш telegram id",
	"хранятся даты",
	"удалите её командой /forget",
	"удаляет вашу связку",
	"без владельца",
	"что остаётся после /forget",
	"под псевдонимом, без вашего telegram id",
	"рассказы ии) — до 30 дней",
	"90 дней",
	"180 дней",
	"в резервных копиях игры",
	"резервная копия связки",
	"по правилам провайдера",
	"хранит telegram",
	"исполнилось 18 лет",
	"работы ии — до 90 дней",
	"статистика — до 180 дней",
}

// typography is the one normalisation of the text and the substrings before
// they are compared (R2-N-2 of T-318): a cosmetic edit of the typography does
// not fail the test, an edit of the meaning does.
var typography = strings.NewReplacer("ё", "е", "\u00a0", " ", "\u202f", " ", "–", "—")

func normalized(s string) string { return typography.Replace(strings.ToLower(s)) }

// noticeProblems checks a notice against the list of T-318 §3 and returns
// what is wrong with it.
func noticeProblems(text string) []string {
	var problems []string
	lower := normalized(text)
	for _, s := range noticeSubstrings {
		if n := strings.Count(lower, normalized(s)); n != 1 {
			problems = append(problems, "«"+s+"» occurs "+strconv.Itoa(n)+" times, want 1")
		}
	}
	if n := strings.Count(lower, "30 дней"); n != 3 {
		problems = append(problems, "«30 дней» occurs "+strconv.Itoa(n)+" times, want 3")
	}
	if n := utf8.RuneCountInString(text); n > 3000 {
		problems = append(problems, "the notice is "+strconv.Itoa(n)+" characters long, want at most 3000")
	}
	return problems
}

func TestTheNoticeHasEveryPointOfFR009Once(t *testing.T) {
	if len(noticeSubstrings) != 24 {
		t.Fatalf("the list has %d substrings, T-318 §3 has 24", len(noticeSubstrings))
	}
	for _, p := range noticeProblems(render.NoticeText) {
		t.Error(p)
	}
}

// The length and the digest pin the accepted text as a whole, its order
// included (tasks/T-318.md §1: 2 341 characters). A deliberate change of the
// text changes them together with the review of T-318.
func TestTheNoticeIsTheAcceptedText(t *testing.T) {
	if n := utf8.RuneCountInString(render.NoticeText); n != 2341 {
		t.Errorf("NoticeText is %d characters, the accepted text is 2341", n)
	}
	sum := sha256.Sum256([]byte(render.NoticeText))
	if got := hex.EncodeToString(sum[:]); got != "f972e1f0b12775d34c5d3f062eafa58e6c76a498a902a7c8c1eeb8935385f543" {
		t.Errorf("NoticeText digest %s differs from the accepted text", got)
	}
	if strings.HasSuffix(render.NoticeText, "\n") || strings.Contains(render.NoticeText, "\r") {
		t.Error("NoticeText ends with a line feed or carries a carriage return")
	}
}

func TestTypographyDoesNotFailTheCheck(t *testing.T) {
	cosmetic := strings.ReplaceAll(render.NoticeText, "ё", "е")
	cosmetic = strings.ReplaceAll(cosmetic, " дней", "\u00a0дней")
	cosmetic = strings.ReplaceAll(cosmetic, "—", "–")
	if cosmetic == render.NoticeText {
		t.Fatal("control: the cosmetic edit changed nothing")
	}
	for _, p := range noticeProblems(cosmetic) {
		t.Errorf("a cosmetic edit fails the check: %s", p)
	}
}

// The mutants of the DoD of T-311: each edit of meaning fails the check.
func TestEditsOfMeaningFailTheCheck(t *testing.T) {
	cut := func(from, to string) string {
		i, j := strings.Index(render.NoticeText, from), strings.Index(render.NoticeText, to)
		if i < 0 || j < i {
			t.Fatalf("control: section %q…%q not found", from, to)
		}
		return render.NoticeText[:i] + render.NoticeText[j:]
	}
	swap := strings.NewReplacer("до 90 дней", "до 180 дней", "до 180 дней", "до 90 дней")
	mutants := map[string]string{
		"section 2 removed":    cut("2. Только для взрослых", "3. Что происходит"),
		"section 6 removed":    cut("6. Что остаётся после /forget", "Перечитать это сообщение"),
		"90 and 180 swapped":   swap.Replace(render.NoticeText),
		"18 → 16 in section 2": strings.Replace(render.NoticeText, "исполнилось 18 лет", "исполнилось 16 лет", 1),
	}
	for name, text := range mutants {
		if text == render.NoticeText {
			t.Errorf("%s: control, the mutant equals the text", name)
			continue
		}
		if len(noticeProblems(text)) == 0 {
			t.Errorf("%s: the check passes an edit of meaning", name)
		}
	}
}

func TestTextsCarryNoMarkup(t *testing.T) {
	texts := map[string]string{
		"NoticeText":    render.NoticeText,
		"DeclineReply":  render.DeclineReply,
		"ConsentButton": render.ConsentButton,
		"DeclineButton": render.DeclineButton,
	}
	for name, text := range texts {
		if i := strings.IndexAny(text, "`*_[<"); i >= 0 {
			t.Errorf("%s carries the markup character %q", name, text[i])
		}
	}
}

// A reply keyboard sends its label back as it is, and the flow compares it
// exactly: an invisible character or a space at an end would make the button
// the player sees differ from the one the flow waits for (N-2 of T-318).
func TestButtonsAreExactPlainLabels(t *testing.T) {
	for name, b := range map[string]string{"ConsentButton": render.ConsentButton, "DeclineButton": render.DeclineButton} {
		if strings.ContainsAny(b, "\u00a0\u200b\u200c\u200d\u2060\ufeff") {
			t.Errorf("%s carries an invisible character: %q", name, b)
		}
		if strings.TrimSpace(b) != b {
			t.Errorf("%s has a space at an end: %q", name, b)
		}
	}
	if n := utf8.RuneCountInString(render.ConsentButton); n > 30 {
		t.Errorf("ConsentButton is %d characters, want at most 30", n)
	}
	if !strings.Contains(render.NoticeText, "«"+render.ConsentButton+"»") {
		t.Error("the notice does not name the consent button word for word")
	}
	// Literals on purpose: the constants are the accepted texts of T-318 §1.
	if render.ConsentButton != "Мне есть 18, принимаю" || render.DeclineButton != "Отказаться" {
		t.Errorf("buttons %q, %q differ from T-318 §1", render.ConsentButton, render.DeclineButton)
	}
}

// The text of FR-009 lives only in notice.go (DoD of T-311): no other file of
// the bot repeats a line of it.
func TestTheNoticeLivesOnlyInNoticeGo(t *testing.T) {
	var lines []string
	for _, line := range strings.Split(render.NoticeText, "\n") {
		if utf8.RuneCountInString(line) >= 30 {
			lines = append(lines, line)
		}
	}
	if len(lines) < 10 {
		t.Fatalf("control: only %d long lines in the notice", len(lines))
	}
	root := filepath.Join("..", "..")
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if filepath.Base(path) == "notice.go" && filepath.Base(filepath.Dir(path)) == "render" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		for _, line := range lines {
			if strings.Contains(string(data), line) {
				t.Errorf("%s repeats a line of the notice: %q", path, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 10 {
		t.Fatalf("control: only %d files of the bot checked", checked)
	}
}
