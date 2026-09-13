package render

// HelpText is the list of commands of FR-002 (component §10.3). A player who
// has consented gets it with NoticeText after it, since the notice says it can
// be reread with /help; a player who has not gets only NoticeText.
const HelpText = `Команды игры:
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
