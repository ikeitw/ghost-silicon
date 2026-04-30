// pkg/renderer/command.go
// Package renderer — command types.
// Commands are high-level instructions the supervisor sends to the renderer
// adapter after launch (navigate, stop, reload, screenshot, etc.).
package renderer

// CommandType classifies a renderer command.
type CommandType string

const (
	CommandNavigate   CommandType = "navigate"
	CommandStop       CommandType = "stop"
	CommandReload     CommandType = "reload"
	CommandGoBack     CommandType = "go_back"
	CommandGoForward  CommandType = "go_forward"
	CommandSetCookies CommandType = "set_cookies"
	CommandClearData  CommandType = "clear_data"
)

// Command is a generic instruction for the renderer.
type Command struct {
	Type    CommandType
	Payload any
}

// NavigatePayload carries the URL for a CommandNavigate command.
type NavigatePayload struct {
	URL            string
	WaitForLoad    bool
	TimeoutSeconds int
}

// ClearDataPayload specifies which storage categories to clear.
type ClearDataPayload struct {
	// Types is a list of "cache", "cookies", "storage", "all".
	Types []string
}

// Navigate builds a navigate command.
func Navigate(url string, wait bool) Command {
	return Command{
		Type: CommandNavigate,
		Payload: NavigatePayload{
			URL:            url,
			WaitForLoad:    wait,
			TimeoutSeconds: 30,
		},
	}
}

// Stop builds a stop command.
func Stop() Command { return Command{Type: CommandStop} }

// Reload builds a reload command.
func Reload() Command { return Command{Type: CommandReload} }
