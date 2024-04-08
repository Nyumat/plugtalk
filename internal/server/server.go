package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"plugtalk/internal/database"

	"golang.org/x/time/rate"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	clientMsgBuffer = 16
	serverMsgBuffer = 20
)

type Server struct {
	port int
	chat *chatServer
	db   database.Service
	host string
}

func NewServer(host string, port int) (*Server, *http.Server) {
	chatServer := newChatServer() // Set up your chat server
	dbService := database.New()   // Set up your database connection

	// Initialize your custom Server struct
	myServer := &Server{
		host: host,
		port: port,
		chat: chatServer,
		db:   dbService,
	}

	// Configure the HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      myServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return myServer, httpServer
}

// chatServer manages all the chat rooms.
// There should only be one instance of it for the site.
type chatServer struct {
	// rooms maps IP address strings to chat rooms
	rooms   map[string]*chatRoom
	roomsMu sync.Mutex

	serveMux http.ServeMux
}

func newChatServer() *chatServer {
	cs := &chatServer{
		rooms: make(map[string]*chatRoom),
	}
	cs.serveMux.HandleFunc("/connect", cs.connectHandler)
	return cs
}

// noCache disables caching of responses.
func noCache(next http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Add("Cache-Control", "no-store, max-age=0")
		rw.Header().Add("Pragma", "no-cache")
		next(rw, r)
	}
}

// noCacheHandler disables caching of responses for http.Handler.
func noCacheHandler(h http.Handler) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Add("Cache-Control", "no-store, max-age=0")
		rw.Header().Add("Pragma", "no-cache")
		h.ServeHTTP(rw, r)
	}
}

func newChatRoom() *chatRoom {
	cr := &chatRoom{
		incoming: make(chan message, serverMsgBuffer),
		quit:     make(chan struct{}),
		clients:  make(map[*client]struct{}),
		// TODO: is this a good limiter?
		limiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
	}
	go cr.start()
	return cr
}

func (cr *chatRoom) start() {
	for {
		select {
		case <-cr.quit:
			return
		case m := <-cr.incoming:
			cr.limiter.Wait(context.Background())

			authorMsg, chatMsg := cr.handleMessage(m)
			if chatMsg == "" {
				// No message needs to be sent to all clients
				continue
			}
			cr.clientsMu.Lock()
			for c := range cr.clients {
				if m.sender == c {
					// This client sent the message, so clear their input field
					c.sendText(authorMsg + clearInputFieldMsg)
				} else {
					c.sendText(chatMsg)
				}
			}
			cr.clientsMu.Unlock()
		}
	}
}

// sendText tries to send the provided string to the client. If the client's
// outgoing channel is full, the client's closeSlow func is called in a goroutine.
func (c *client) sendText(s string) {
	select {
	case c.outgoing <- s:
	default:
		go c.closeSlowly()

	}
}

// addClient adds a client to the approriate chat room, creating it if needed.
// The room the client is in is returned. It also generates and sets a nickname
// for the client.
func (cs *chatServer) addClient(ip string, c *client) *chatRoom {
	cs.roomsMu.Lock()
	defer cs.roomsMu.Unlock()
	room, ok := cs.rooms[ip]
	if !ok {

		room = newChatRoom()
		cs.rooms[ip] = room
	}

	// Nickname generation happens inside the room func
	room.addClient(c)

	// Insert room name
	c.outgoing <- fmt.Sprintf(`<h2 id="ip-addr">%s</h2>`, ip)

	return room
}

// removeClient removes a client from the approriate chat room, removing the
// entire chat room if it's empty.
func (cs *chatServer) removeClient(ip string, c *client) {
	cs.roomsMu.Lock()
	defer cs.roomsMu.Unlock()

	room, ok := cs.rooms[ip]
	if !ok {
		// Room doesn't exist, so ignore
		log.Printf("chatServer.removeClient: Tried to remove client from non-existent room %s", ip)
		return
	}
	room.removeClient(c)

	if room.numClients() == 0 {
		delete(cs.rooms, ip)
		room.quit <- struct{}{}
	}
}

func (cs *chatServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cs.serveMux.ServeHTTP(w, r)
}

// connectHandler accepts the WebSocket connection and sets up the duplex messaging.
func (cs *chatServer) connectHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("subscribeHandler: Websocket accept error: %v", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "")

	err = cs.connect(r.Context(), getIPString(r), conn)
	if errors.Is(err, context.Canceled) {
		return
	}

	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return
	}
	if err != nil {
		log.Printf("chatServer.connectHandler: %v", err)
		return
	}
}

// htmxJson decodes a JSON websocket message from the web UI, which uses htmx (htmx.org)
// This is the message sent when the user sends a message.
type htmxJson struct {
	Msg     string                 `json:"message"`
	Headers map[string]interface{} `json:"HEADERS"`
}

// connect creates a client and passes messages to and from it.
// If the context is cancelled or an error occurs, it returns and removes the client.
func (cs *chatServer) connect(ctx context.Context, ip string, conn *websocket.Conn) error {
	cl := &client{
		outgoing: make(chan string, clientMsgBuffer),
		closeSlowly: func() {
			conn.Close(websocket.StatusPolicyViolation, "connection too slow to keep up with messages")
		},
	}
	room := cs.addClient(ip, cl)
	defer cs.removeClient(ip, cl)

	// Read websocket messages from user into channel
	// Cancel context when connection is closed
	readCh := make(chan string, serverMsgBuffer)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			var webMsg htmxJson
			err := wsjson.Read(ctx, conn, &webMsg)
			if err != nil {
				// Treat any error the same as it being closed
				cancel()
				conn.Close(websocket.StatusPolicyViolation, "unexpected error")
				return
			}
			readCh <- webMsg.Msg
		}
	}()

	for {
		select {
		case text := <-cl.outgoing:
			// Send message to user
			err := writeTimeout(ctx, time.Second*5, conn, text)
			if err != nil {
				return err
			}
		case text := <-readCh:
			// Send message to chat room
			room.incoming <- message{
				nickname: cl.nickname,
				text:     text,
				sender:   cl,
				sentAt:   time.Now(),
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func getIPString(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		forwardedIPs := strings.Split(xForwardedFor, ",")
		if len(forwardedIPs) > 0 {
			// Trim any accidental whitespace from IP strings.
			realIP := strings.TrimSpace(forwardedIPs[len(forwardedIPs)-1])
			log.Printf("Detected reverse-proxied IP: %s", realIP)
			return realIP
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		log.Printf("getIPString: Error splitting host and port: %v", err)
		return r.RemoteAddr // Fallback, but consider if this is appropriate for your use case
	}

	return ip
}

func writeTimeout(ctx context.Context, timeout time.Duration, conn *websocket.Conn, text string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return conn.Write(ctx, websocket.MessageText, []byte(text))
}
