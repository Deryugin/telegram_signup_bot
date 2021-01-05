package main

import (
	"fmt"
	"github.com/Syfaro/telegram-bot-api"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var bot *tgbotapi.BotAPI

type User struct {
	Username string
	Nickname string
	Uid      int
}

func htmlEscape(txt string) string {
	re := regexp.MustCompile("&")
	txt = re.ReplaceAllString(txt, "&amp;")

	re = regexp.MustCompile("<")
	txt = re.ReplaceAllString(txt, "&lt;")

	re = regexp.MustCompile(">")
	txt = re.ReplaceAllString(txt, "&gt;")

	return txt
}

func (u User) toString() string {
	res := "<a href=\"tg://user?id=" + strconv.Itoa(u.Uid) + "\">" + htmlEscape(u.Nickname) + "(@" + u.Username + ")</a>"

	return res
}

type SignupOption struct {
	Header      string
	Text        string
	SignedUsers map[int]User
}

func (s SignupOption) toString() string {
	res := "<b>" + s.Header + "</b>\n" + s.Text + "\n"
	i := 0
	for uid := range s.SignedUsers {
		res += strconv.Itoa(i+1) + ". " + s.SignedUsers[uid].toString() + "\n"
		i++
	}
	return res
}

type SignupMessage struct {
	Header  string
	Options []SignupOption
}

func (s SignupMessage) toString() string {
	res := "<b>" + s.Header + "</b>\n\n"

	for _, o := range s.Options {
		res += o.toString() + "\n"
	}

	return res
}

func (s SignupMessage) keyboard() tgbotapi.InlineKeyboardMarkup {
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for i, o := range s.Options {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(o.Header, strconv.Itoa(i)))
		keyboard = append(keyboard, row)
	}
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func doReply(replyTo tgbotapi.Message, text string) tgbotapi.Message {
	msg := tgbotapi.NewMessage(replyTo.Chat.ID, text)
	msg.ReplyToMessageID = replyTo.MessageID
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true
	m, _ := bot.Send(msg)
	return m
}

var signups = make(map[int]SignupMessage)

func main() {
	fmt.Printf("===============================================\n" +
		"Добро пожаловать в мир цифрового животноводства\n" +
		"===============================================\n")
	var err error

	bot_token := os.Getenv("BOT_TOKEN")
	if bot_token == "" {
		log.Panic("No token provided")
		return
	}

	bot, err = tgbotapi.NewBotAPI(bot_token)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			ds := update.CallbackQuery.Data
			option, _ := strconv.Atoi(ds)
			msg := update.CallbackQuery.Message
			signupMessage := signups[msg.MessageID]
			user := update.CallbackQuery.From
			log.Printf("Callback: %s %s; mid: %d", user.UserName, ds, msg.MessageID)
			signedUser := User{Username: user.UserName, Uid: user.ID,
				Nickname: user.FirstName + " " + user.LastName}
			_, exists := signupMessage.Options[option].SignedUsers[user.ID]
			if exists {
				delete(signupMessage.Options[option].SignedUsers, user.ID)
			} else {
				signupMessage.Options[option].SignedUsers[user.ID] = signedUser
			}
			kbd := signupMessage.keyboard()
			edit := tgbotapi.EditMessageTextConfig{
				BaseEdit: tgbotapi.BaseEdit{
					ChatID:      msg.Chat.ID,
					MessageID:   msg.MessageID,
					ReplyMarkup: &kbd,
				},
				Text:      signupMessage.toString(),
				ParseMode: "HTML",
			}
			bot.Send(edit)
		}
		if update.Message == nil {
			continue
		}

		msg := *update.Message
		chat_id := msg.Chat.ID
		from_id := msg.From.ID

		log.Printf("[%s %d:%d] %s", msg.From.UserName, chat_id, from_id, msg.Text)

		if msg.IsCommand() && (chat_id == int64(from_id) || bot.IsMessageToMe(msg)) {
			switch msg.Command() {
			case "start", "help":
				doReply(msg, replies["help"])
			case "create":
				if msg.ReplyToMessage == nil {
					doReply(msg, replies["help"])
					continue
				}
				splits := strings.Split(msg.ReplyToMessage.Text, "\n\n")
				newSignup := SignupMessage{Header: splits[0]}
				for _, line := range splits[1:] {
					subsplit := strings.SplitN(line, "\n", 2)
					fmt.Println(subsplit)
					newOption := SignupOption{Header: subsplit[0]}
					if len(subsplit) > 1 {
						newOption.Text = subsplit[1]
					}
					newOption.SignedUsers = make(map[int]User)
					newSignup.Options = append(newSignup.Options, newOption)
				}

				m := tgbotapi.NewMessage(chat_id, newSignup.toString())
				m.ReplyToMessageID = msg.MessageID
				m.ReplyMarkup = newSignup.keyboard()
				m.ParseMode = "HTML"
				m.DisableWebPagePreview = true
				reply, _ := bot.Send(m)

				bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
					ChatID:    msg.Chat.ID,
					MessageID: msg.ReplyToMessage.MessageID,
				})
				bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
					ChatID:    msg.Chat.ID,
					MessageID: msg.MessageID,
				})

				signups[reply.MessageID] = newSignup

				pinChatMessageConfig := tgbotapi.PinChatMessageConfig{
					ChatID:              reply.Chat.ID,
					MessageID:           reply.MessageID,
					DisableNotification: true,
				}
				bot.PinChatMessage(pinChatMessageConfig)
			}
		}
	}
}
