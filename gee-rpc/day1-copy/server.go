package geerpc

type Server struct{}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{}
}

const MagicNumber = 0x3bef5c
