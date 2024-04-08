package server

import (
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"
)

type message struct {
	nickname string // empty -> server message else, user message
	text     string
	sender   *client // nil -> server message else, user message
	sentAt   time.Time
	raw      string
}

const (
	maxNicknameLen = 30
	maxMsgTextLen  = 512
)

// createUserListMsg creates HTML that can replace the current user list.
// It assume the nicknames provided are already HTML escaped.
func createUserListMsg(nicks []string) string {
	log.Println("called user list")
	var b strings.Builder
	b.WriteString(`<div id="users-list">`)
	for i := range nicks {
		b.WriteString(fmt.Sprintf(`<p>%s</p>`, nicks[i]))
	}
	b.WriteString(`</div>`)
	b.WriteString(fmt.Sprintf(`<p id="users-header-p" class="bold">Users (%d)</p>`, len(nicks)))
	return b.String()
}

// createSpecialMsg creates a message not from any specific user, that has a
// CSS class. This can be used for error messages, or notifications.
func createSpecialMsg(text string, class string) string {
	var ts string
	if class == "notif" {
		// Notification messages are timestamped
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf(
		// Add message to log
		`<tbody id="message-table-tbody" hx-swap-oob="beforeend">
			<tr class="special-message"><td>%s</td><td></td><td class="%s">%s</td></tr>
		</tbody>`,
		ts, class, html.EscapeString(text),
	)
}

// createJoinMsg creates a message struct that can be sent to a chat room sentAt a client joins.
func createJoinMsg(c *client, nicks []string) message {
	log.Println("called join msg")
	return message{
		raw: createSpecialMsg(fmt.Sprintf("%s has joined", c.nickname), "notif") +
			createUserListMsg(nicks),
		sentAt: time.Now(),
	}
}

// createLeaveMsg creates a message struct that can be sent to a chat room sentAt a client leaves.
func createLeaveMsg(c *client, nicks []string) message {
	return message{
		raw: createSpecialMsg(fmt.Sprintf("%s has left", c.nickname), "notif") +
			createUserListMsg(nicks),
		sentAt: time.Now(),
	}
}

func sanitizeNick(nickname string) string {
	nickname = strings.ToValidUTF8(nickname, "\uFFFD")
	nickname = strings.TrimSpace(nickname)
	// Unicode normalization, to prevent look-alike nicknames
	nickname = norm.NFC.String(nickname)

	// Truncate by graphemes instead of runes, so multi-rune things like flags work
	g := uniseg.NewGraphemes(nickname)
	i := 0
	nickname = ""
	for g.Next() && i < maxNicknameLen {
		nickname += g.Str()
		i++
	}

	nickname = html.EscapeString(nickname)
	return nickname
}

var urlRe = regexp.MustCompile(`(?i)\b(?:[a-z][\w.+-]+:(?:/{1,3}|[?+]?[a-z0-9%]))(?:[^\s()<>]+|\(([^\s()<>]+|(\([^\s()<>]+\)))*\))+(?:\(([^\s()<>]+|(\([^\s()<>]+\)))*\)|[^\s\x60!()\[\]{};:'".,<>?«»“”‘’])`)

func renderMsgText(text string) string {
	text = strings.ToValidUTF8(text, "\uFFFD")
	text = strings.TrimSpace(text)

	// TODO: is this too slow?
	g := uniseg.NewGraphemes(text)
	i := 0
	var b strings.Builder
	for g.Next() && i < maxMsgTextLen {
		b.Write(g.Bytes())
		i++
	}
	text = b.String()
	text = html.EscapeString(text)

	// Linkify URLs
	text = urlRe.ReplaceAllStringFunc(text, func(urlText string) string {
		return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, urlText, urlText)
	})

	return text
}

func validateMessageText(s string) bool {
	return s != ""
}

func createChatMsg(m message) (string, string) {
	sanitizedMsgText := renderMsgText(m.text)
	if !validateMessageText(sanitizedMsgText) {
		return "", ""
	}

	// Format the timestamp into a more human-readable form if necessary
	ts := m.sentAt.Local().Format("15:04")

	// Differentiate styling between the author and non-author
	authorHTML := fmt.Sprintf(
		`<div class="chat chat-start" id="author-chat" hx-swap-oob="beforeend">
			<time class="text-xs opacity-50">%s</time>
			<span class="font-bold" id="nickname">%s</span>
            <div>%s</div>
        </div>`, ts, m.nickname, sanitizedMsgText,
	)

	nonAuthorHTML := fmt.Sprintf(
		`<div class="chat chat-start" id="author-chat" hx-swap-oob="beforeend">
			<time class="text-xs opacity-50">%s</time>
			<span class="font-bold" id="nickname">%s</span>
            <div>%s</div>
        </div>`, ts, m.nickname, sanitizedMsgText,
	)
	return authorHTML, nonAuthorHTML
}

func (cr *chatRoom) handleMessage(m message) (string, string) {
	cr.clientsMu.Lock()
	defer cr.clientsMu.Unlock()

	if m.raw != "" {
		// Message is already rendered
		return m.raw, m.raw
	}

	if strings.HasPrefix(m.text, "/nickname ") && len(m.text) > len("/nickname ") {
		newNick := sanitizeNick(m.text[len("/nickname "):])
		if newNick == "" {
			// Empty nickname, invalid
			m.sender.forwardMessage(createSpecialMsg("Nickname cannot be empty", "error"))
			return "", ""
		}
		if cr.nickNameInUse(newNick) {
			m.sender.forwardMessage(createSpecialMsg("That nickname is already in use", "error"))
			return "", ""
		}
		oldNick := m.sender.nickname
		m.sender.nickname = newNick
		// Tell everyone about name change, and update user list
		s := createSpecialMsg(
			fmt.Sprintf("%s is now known as %s", oldNick, newNick), "notif",
		) +
			createUserListMsg(cr.nicks())
		return s, s
	}

	// Regular message
	cr.whenLastMsg = m.sentAt
	return createChatMsg(m)
}
