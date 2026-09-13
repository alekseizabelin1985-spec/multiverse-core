package render_test

import (
	"strings"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/internal/gateway/api"
)

// The golden texts are literals in this file, not files under testdata: a
// literal is changed by a reviewed edit only, never by a flag that rewrites
// golden files together with the code (T-318 §3).

func message(code string) string {
	spec, ok := api.LookupError(code)
	if !ok {
		panic("no such code " + code)
	}
	return spec.Message
}

func TestGoldenErrorTexts(t *testing.T) {
	cases := []struct {
		code       string
		retryAfter time.Duration
		want       string
	}{
		{api.CodeInEncounter, 0, "Сейчас идёт бой. Сначала /flee."},
		{api.CodeNotInEncounter, 0, "Сейчас нет боя. Вокруг никого; /look."},
		{api.CodeNotLeader, 0, "Это может сделать только лидер группы. Регион меняет лидер группы."},
		{api.CodeAlreadyActed, 0, "В этом раунде вы уже действовали. Дождитесь конца раунда."},
		{api.CodeCharacterDead, 0, "Персонаж больше не может действовать. /start — создать нового."},
		{api.CodeConsentRequired, 0, "Сначала нужно дать согласие. Прочитайте условия и нажмите «Мне есть 18, принимаю»."},
		{api.CodeRateLimited, 0, "Слишком часто, подождите."},
		{api.CodeRateLimited, 12 * time.Second, "Слишком часто, подождите 12 с."},
		{api.CodeBusUnavailable, 0, "Сервис недоступен, повторите позже."},
		{api.CodeStateUnavailable, 0, "Сервис недоступен, повторите позже."},
		{api.CodeForgetIncomplete, 5 * time.Second, "Команда /forget ещё не завершена. Повторите /forget confirm через 5 с."},
		{api.CodeForgetIncomplete, 0, "Команда /forget ещё не завершена. Повторите /forget confirm чуть позже."},
	}
	for _, c := range cases {
		if got := render.ErrorText(c.code, message(c.code), c.retryAfter); got != c.want {
			t.Errorf("%s (Retry-After %v):\n got %q\nwant %q", c.code, c.retryAfter, got, c.want)
		}
	}
}

func TestAnUnknownCodeShowsTheMessageOfTheServer(t *testing.T) {
	if got := render.ErrorText("brand_new_code", "Что-то новое на сервере.", 0); got != "Что-то новое на сервере." {
		t.Errorf("unknown code = %q, want the message of the server", got)
	}
	if got := render.ErrorText(api.CodeTargetDead, "", 0); got != "Цель уже повержена." {
		t.Errorf("known code without a message = %q, want the message of the error table", got)
	}
	if got := render.ErrorText("brand_new_code", "", 0); got == "" {
		t.Error("unknown code without a message gives an empty text")
	}
}

// Mi-4 of review #1 of T-311: an answer that is not of the contract — no code,
// the message is the HTTP status text of a proxy — reads as the service being
// unavailable, never as "Bad Gateway" or a raw status.
func TestAnAnswerWithoutACodeReadsAsUnavailable(t *testing.T) {
	for _, message := range []string{"Bad Gateway", "Service Unavailable", "Gateway Timeout", "Not Found", "", "502"} {
		if got := render.ErrorText("", message, 0); got != render.Unavailable {
			t.Errorf("no code, message %q = %q, want %q", message, got, render.Unavailable)
		}
	}
}

// forget_incomplete never says "deleted": the link is gone but its bytes are
// not wiped yet (C-08 v1.5, I-2 of T-318). The message of the gateway for this
// code does say it, so the control checks the test would catch a text built on
// that message.
func TestForgetIncompleteNeverSaysDeleted(t *testing.T) {
	if !strings.Contains(strings.ToLower(message(api.CodeForgetIncomplete)), "удален") {
		t.Fatal("control: the message of the gateway no longer mentions a deletion; the check proves nothing")
	}
	for _, d := range []time.Duration{0, time.Second, 5 * time.Second, 90 * time.Second} {
		for _, text := range []string{
			render.ErrorText(api.CodeForgetIncomplete, message(api.CodeForgetIncomplete), d),
			render.ForgetIncomplete(d),
		} {
			if strings.Contains(strings.ToLower(text), "удален") {
				t.Errorf("Retry-After %v: %q says the data are deleted", d, text)
			}
		}
	}
}

func TestGoldenDeclineReply(t *testing.T) {
	const want = "Без согласия и подтверждения возраста играть нельзя, персонаж не создан. Если передумаете, отправьте /start. Чтобы бот удалил ваш Telegram ID, отправьте /forget."
	if render.DeclineReply != want {
		t.Errorf("DeclineReply:\n got %q\nwant %q", render.DeclineReply, want)
	}
}

func TestGoldenHelp(t *testing.T) {
	const want = `Команды игры:
/start — начать игру или вернуться к персонажу
/status — состояние персонажа
/enter регион — войти в регион
/leave — выйти из региона
/look — осмотреться
/attack цель — атаковать
/defend — защищаться в бою
/flee — сбежать из боя
/rest — отдохнуть вне боя
/say текст — сказать вслух, до 500 символов
/group create, /group join номер, /group leave — группа
/help — эта справка и условия игры
/forget — удалить связку с игрой, бот попросит подтвердить`
	if render.HelpText != want {
		t.Errorf("HelpText:\n got %q\nwant %q", render.HelpText, want)
	}
	for _, cmd := range []string{"/start", "/status", "/enter", "/leave", "/look", "/attack", "/defend", "/flee", "/rest", "/say", "/group", "/help", "/forget"} {
		if !strings.Contains(render.HelpText, cmd) {
			t.Errorf("HelpText does not name %s (FR-002)", cmd)
		}
	}
}

func TestGoldenForgetTexts(t *testing.T) {
	if render.ForgetDone != "Связка удалена. /start — начать заново" {
		t.Errorf("ForgetDone = %q", render.ForgetDone)
	}
	if !strings.Contains(render.ForgetQuestion, "/forget confirm") {
		t.Errorf("ForgetQuestion = %q, want it to name /forget confirm", render.ForgetQuestion)
	}
	if strings.Contains(strings.ToLower(render.ForgetNothing), "удален") {
		t.Errorf("ForgetNothing = %q says something was deleted", render.ForgetNothing)
	}
}

func TestGoldenStatus(t *testing.T) {
	hp, max := 7, 10
	st := api.CharacterState{
		PlayerID: "player-A", Name: "Вася", WorldID: "dark-forest-world", Status: "alive", HP: &hp, HPMax: &max,
		Position:  &api.PositionRef{Kind: "region", ID: "dark-forest-01", Name: "Тёмный лес"},
		Encounter: &api.EncounterView{EncounterID: "enc-7", NPCs: []api.NPCView{{NPCID: "wolf-alpha", Name: "Альфа-волк", HP: 4, HPMax: 10, Status: "alive"}, {NPCID: "wolf-beta", Name: "Волк", Status: "dead"}}},
		Inventory: []api.Item{{ItemID: "item-1", Kind: "wolf-pelt", Name: "волчья шкура"}},
	}
	const want = "«Вася»: жив, HP 7/10\nГде: Тёмный лес\nБой: Альфа-волк 4/10 — /attack wolf-alpha; Волк (повержен)\nИнвентарь: волчья шкура"
	if got := render.Status(st); got != want {
		t.Errorf("Status:\n got %q\nwant %q", got, want)
	}
	r := render.StatusReply(st)
	if r.Keyboard == nil || r.Keyboard.Rows[0][0] != "/attack wolf-alpha" {
		t.Errorf("StatusReply in an encounter keyboard = %+v, want the combat keyboard", r.Keyboard)
	}
	st.Encounter = nil
	if r := render.StatusReply(st); r.Keyboard == nil || r.Keyboard.Rows[0][0] != "/look" {
		t.Errorf("StatusReply outside an encounter keyboard = %+v, want the base keyboard", r.Keyboard)
	}
}
