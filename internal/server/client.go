package server

type client struct {
	nickname    string      // sanitized nickname of the client (user)
	outgoing    chan string // receives outgoing pre-rendered messages
	closeSlowly func()      // close the client slowly
}

func (c *client) forwardMessage(message string) {
	select {
	case c.outgoing <- message:
	default:
		go c.closeSlowly()

	}
}
