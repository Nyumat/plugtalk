package server

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"plugtalk/internal/shared"

	"golang.org/x/time/rate"
)

type chatRoom struct {
	// incoming is where messages sent by clients are temporarily stored.
	incoming chan message
	// quit is used to stop the chatRoom goroutine
	quit chan struct{}
	// limiter rate limits the messages sent to the server for this room.
	// This prevents the server from being spammed by messages.
	limiter *rate.Limiter
	// whenLastMsg is when the most recent message was sent
	whenLastMsg time.Time

	clientsMu sync.Mutex
	clients   map[*client]struct{} // map is used for easy removal
}

func createChatRoom() *chatRoom {
	return &chatRoom{
		incoming: make(chan message),
		quit:     make(chan struct{}),
		limiter:  rate.NewLimiter(rate.Every(time.Second), 5),
		clients:  make(map[*client]struct{}),
	}
}

// addClient adds a client to the chat room.
// It also generates a nickname for them.
// The chatServer addClient method should be used by clients instead.
func (cr *chatRoom) addClient(c *client) {
	cr.clientsMu.Lock()
	defer cr.clientsMu.Unlock()
	c.nickname = cr.getNewNick()
	cr.clients[c] = struct{}{}
	cr.incoming <- createJoinMsg(c, cr.nicks())
}

// removeClient removes a client from the chat room.
// The chatServer removeClient method should be used by clients instead.
func (cr *chatRoom) removeClient(c *client) {
	cr.clientsMu.Lock()
	defer cr.clientsMu.Unlock()
	delete(cr.clients, c)
	if len(cr.clients) > 0 {
		// Send leave message to clients left in the room
		cr.incoming <- createLeaveMsg(c, cr.nicks())
	}
}

// numClients returns the number of clients in the room.
// It holds the client mutex.
func (cr *chatRoom) numClients() int {
	cr.clientsMu.Lock()
	defer cr.clientsMu.Unlock()
	return len(cr.clients)
}

const clearInputFieldMsg = `<input name="message" id="message-input" type="text" />`

func (cr *chatRoom) init() {
	for {
		select {
		case <-cr.quit:
			return
		case m := <-cr.incoming:
			// Handle user messages
			if err := cr.limiter.Wait(context.Background()); err != nil {
				log.Printf("Error waiting for rate limiter: %v", err)
				// handle the error, for example, you might want to continue to the next iteration of the loop
				continue
			}

			if !cr.limiter.Allow() {
				// Drop the message if the rate limiter is blocking it
				continue
			}

			// Handle server messages
			if m.sender == nil {
				// This is a server message
				cr.clientsMu.Lock()
				for c := range cr.clients {
					c.forwardMessage(m.text)
				}
				cr.clientsMu.Unlock()
				continue
			}

			authorMsg, chatMsg := cr.handleMessage(m)
			if chatMsg == "" {
				// No message needs to be sent to all clients
				continue
			}
			cr.clientsMu.Lock()
			for c := range cr.clients {
				if m.sender == c {
					// This client sent the message, so clear their input field
					c.forwardMessage(authorMsg + clearInputFieldMsg)
				} else {
					c.forwardMessage(chatMsg)
				}
			}
			cr.clientsMu.Unlock()
		}
	}
}

// Check if the nickname is already in use
func (cr *chatRoom) nickNameInUse(nick string) bool {
	for c := range cr.clients {
		if nick == c.nickname {
			return true
		}
	}
	return false
}

// Returns a new nickname that is not already in use
// Thread-safe with respect to the clients mutex
func (cr *chatRoom) getNewNick() string {
	ogNick := shared.GenerateNickname()
	nick := ogNick
	i := 2
	for cr.nickNameInUse(nick) {
		nick = fmt.Sprintf("%s%d", ogNick, i)
		i++
	}
	return nick
}

// nicks returns all the nicknames currently in use in this chat room.
// The nicknames are sorted alphabetically.
// TODO:Don't force callers to be thread-safe
func (cr *chatRoom) nicks() []string {
	nickNames := make([]string, len(cr.clients))
	i := 0
	for c := range cr.clients {
		nickNames[i] = c.nickname
		i++
	}
	sort.Strings(nickNames)
	return nickNames
}
