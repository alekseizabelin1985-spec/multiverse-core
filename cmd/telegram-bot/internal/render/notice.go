// Package render holds every text the bot shows a player and the keyboards
// it shows them with: the notice of FR-009, the help, the onboarding prompts,
// the hints on error codes and the rendering of deliveries (component §10.3–
// §10.5). Every text is plain: the bot sends without parse_mode (SEC-10).
package render

// The notice of FR-009 and its buttons, word for word as accepted in T-318
// (tasks/T-318.md §1, SEC-26). This file is the only place of the text: a
// change of its meaning goes through business-analyst and security-engineer,
// and the tests of notice_test.go fail on it (FR-009, T-318 §3).
//
// ConsentButton and DeclineButton are at once the labels of the reply keyboard
// and what the flow compares an incoming message with, exactly: a reply
// keyboard sends its label back as a message (component §10.3).
const (
	// NoticeText is the first message of /start and the repeat of /help.
	NoticeText = `Здравствуйте! Перед началом игры прочитайте, пожалуйста, как она устроена.

1. Тексты создаёт ИИ
Описания, сюжет и ответы персонажей пишет искусственный интеллект (ИИ). Бои и броски кубиков считаются по правилам игры.

2. Только для взрослых
Играть можно, только если вам исполнилось 18 лет. Нажимая кнопку согласия, вы подтверждаете свой возраст.

3. Что происходит с вашим текстом
Всё, что вы вводите (имя персонажа, реплики), ИИ использует, чтобы вести игру. По умолчанию текст обрабатывается только на компьютере оператора — человека, который запустил эту игру.
Оператор может включить облачную модель ИИ. Тогда её провайдеру, в том числе за рубеж, будут передаваться ваши тексты — и новые, и написанные раньше, — но без вашего Telegram ID. В этом случае бот предупредит вас отдельно.

4. Что хранится
С первого вашего сообщения боту хранится ваш Telegram ID, а когда вы создадите персонажа, к нему добавляется псевдоним игрока — это связка. В ней же хранятся даты, когда вы впервые и в последний раз написали боту, получили это сообщение и дали согласие. Имя и ник из профиля Telegram не сохраняются. Связка хранится, пока вы не удалите её командой /forget или её не удалит оператор.

5. Что делает /forget (бот попросит подтвердить)
• сразу удаляет вашу связку;
• отменяет сообщения, которые ещё не дошли до вас;
• ваш персонаж покидает группу, если был в ней, и остаётся в мире под своим именем, но без владельца. Вернуть его нельзя: после нового /start вы начнёте с новым персонажем.

6. Что остаётся после /forget
Под псевдонимом, без вашего Telegram ID, остаются записи, которые удаляются автоматически. Срок считается с момента записи:
• события игры (ваши реплики, рассказы ИИ) — до 30 дней;
• записи работы ИИ — до 90 дней;
• обезличенная статистика — до 180 дней.
В резервных копиях игры эти записи могут храниться ещё до 30 дней сверх указанных сроков.
Зашифрованная резервная копия связки (с Telegram ID) хранится не дольше 30 дней.
Если оператор включал облачную модель, тексты, уже переданные её провайдеру, хранятся по правилам провайдера — /forget их не удаляет.
Историю переписки с ботом хранит Telegram, и /forget её не удаляет — удалите чат сами.

Перечитать это сообщение можно командой /help.

Если вам есть 18 лет и вы согласны с этими условиями, нажмите «Мне есть 18, принимаю». Без этого создать персонажа нельзя.`

	// ConsentButton confirms the age and the consent with one press.
	ConsentButton = `Мне есть 18, принимаю`

	// DeclineButton declines; the flow answers DeclineReply (decision Р-3 A).
	DeclineButton = `Отказаться`

	// DeclineReply is the answer to DeclineButton: variant A of Р-2, where
	// /forget is accepted before the consent.
	DeclineReply = `Без согласия и подтверждения возраста играть нельзя, персонаж не создан. Если передумаете, отправьте /start. Чтобы бот удалил ваш Telegram ID, отправьте /forget.`
)
