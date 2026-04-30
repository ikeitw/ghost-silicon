package audit

import "time"

// Builder constructs an Event fluently before calling Logger.Log.
type Builder struct {
	ev Event
}

// NewEvent starts building an audit Event of the given type.
func NewEvent(t EventType) *Builder {
	return &Builder{ev: Event{
		Timestamp: time.Now().UTC(),
		Type:      t,
	}}
}

func (b *Builder) Session(id string) *Builder { b.ev.SessionID = id; return b }
func (b *Builder) Profile(id string) *Builder { b.ev.ProfileID = id; return b }
func (b *Builder) Actor(a string) *Builder    { b.ev.Actor = a; return b }
func (b *Builder) Target(t string) *Builder   { b.ev.Target = t; return b }
func (b *Builder) Outcome(o string) *Builder  { b.ev.Outcome = o; return b }
func (b *Builder) Message(m string) *Builder  { b.ev.Message = m; return b }

// Attr adds a single key/value attribute.
func (b *Builder) Attr(key, val string) *Builder {
	if b.ev.Attributes == nil {
		b.ev.Attributes = make(map[string]string)
	}
	b.ev.Attributes[key] = val
	return b
}

// Build returns the finished Event.
func (b *Builder) Build() Event { return b.ev }

// Write sends the event directly to l.
func (b *Builder) Write(l *Logger) { l.Log(b.ev) }
