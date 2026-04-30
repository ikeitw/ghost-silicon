package jsonrpc

import "fmt"

// Router groups related RPC handlers under a namespace prefix.
// For example, a Router with prefix "hardware" registers methods like
// "hardware.getCPUCores", "hardware.getRAM", etc.
type Router struct {
	prefix string
	server *Server
}

// NewRouter creates a Router that registers methods on server with the given
// prefix.  prefix should not include a trailing dot.
func NewRouter(prefix string, server *Server) *Router {
	return &Router{prefix: prefix, server: server}
}

// Handle registers a method under the router's prefix.
// The method name will be "<prefix>.<method>".
func (r *Router) Handle(method string, h HandlerFunc) {
	full := r.prefix + "." + method
	r.server.Register(full, h)
}

// Sub returns a child router with an additional namespace level.
// E.g. router.Sub("screen") on a "hardware" router produces "hardware.screen".
func (r *Router) Sub(name string) *Router {
	return &Router{
		prefix: fmt.Sprintf("%s.%s", r.prefix, name),
		server: r.server,
	}
}
