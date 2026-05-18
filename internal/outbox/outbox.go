package outbox

import (
	"strings"
	"sync"
	"time"
)

type PendingMessage struct {
	ID          string
	Source      string
	Destination string
	Message     string
	CreatedAt   time.Time
}

type Outbox struct {
	mu        sync.Mutex
	ttl       time.Duration
	onFailure func(PendingMessage)
	nextID    int64
	pending   map[string]PendingMessage
	byKey     map[string][]string
	timers    map[string]*time.Timer
}

func New(ttl time.Duration, onFailure func(PendingMessage)) *Outbox {
	return &Outbox{
		ttl:       ttl,
		onFailure: onFailure,
		pending:   make(map[string]PendingMessage),
		byKey:     make(map[string][]string),
		timers:    make(map[string]*time.Timer),
	}
}

func (o *Outbox) Register(source, destination, message string, now time.Time) PendingMessage {
	if o == nil {
		return PendingMessage{}
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.nextID++
	pending := PendingMessage{
		ID:          time.Now().UTC().Format("20060102150405") + "-" + strings.TrimSpace(source) + "-" + stringID(o.nextID),
		Source:      strings.ToUpper(strings.TrimSpace(source)),
		Destination: strings.TrimSpace(destination),
		Message:     message,
		CreatedAt:   now.UTC(),
	}

	o.pending[pending.ID] = pending
	key := matchKey(pending.Source, pending.Destination, pending.Message)
	o.byKey[key] = append(o.byKey[key], pending.ID)
	if o.ttl > 0 {
		o.timers[pending.ID] = time.AfterFunc(o.ttl, func() {
			o.expire(pending.ID)
		})
	}

	return pending
}

func (o *Outbox) Confirm(source, destination, message string) bool {
	if o == nil {
		return false
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	origin := strings.ToUpper(strings.TrimSpace(strings.SplitN(source, ",", 2)[0]))
	key := matchKey(origin, strings.TrimSpace(destination), message)
	ids := o.byKey[key]
	for len(ids) > 0 {
		id := ids[0]
		ids = ids[1:]
		pending, ok := o.pending[id]
		if !ok {
			continue
		}
		delete(o.pending, id)
		if timer := o.timers[id]; timer != nil {
			timer.Stop()
			delete(o.timers, id)
		}
		if len(ids) == 0 {
			delete(o.byKey, key)
		} else {
			o.byKey[key] = ids
		}
		return pending.ID != ""
	}
	delete(o.byKey, key)

	var matched PendingMessage
	for _, pending := range o.pending {
		if pending.Source != origin {
			continue
		}
		if pending.Destination != strings.TrimSpace(destination) {
			continue
		}
		if !messageMatches(pending.Message, message) {
			continue
		}
		if matched.ID == "" || pending.CreatedAt.Before(matched.CreatedAt) {
			matched = pending
		}
	}
	if matched.ID == "" {
		return false
	}
	o.removeLocked(matched)
	return true
}

func (o *Outbox) expire(id string) {
	o.mu.Lock()
	pending, ok := o.pending[id]
	if !ok {
		o.mu.Unlock()
		return
	}
	delete(o.pending, id)
	delete(o.timers, id)
	key := matchKey(pending.Source, pending.Destination, pending.Message)
	o.byKey[key] = removeID(o.byKey[key], id)
	if len(o.byKey[key]) == 0 {
		delete(o.byKey, key)
	}
	onFailure := o.onFailure
	o.mu.Unlock()

	if onFailure != nil {
		onFailure(pending)
	}
}

func (o *Outbox) removeLocked(pending PendingMessage) {
	delete(o.pending, pending.ID)
	if timer := o.timers[pending.ID]; timer != nil {
		timer.Stop()
		delete(o.timers, pending.ID)
	}
	key := matchKey(pending.Source, pending.Destination, pending.Message)
	o.byKey[key] = removeID(o.byKey[key], pending.ID)
	if len(o.byKey[key]) == 0 {
		delete(o.byKey, key)
	}
}

func matchKey(source, destination, message string) string {
	return strings.ToUpper(strings.TrimSpace(source)) + "\x00" + strings.TrimSpace(destination) + "\x00" + message
}

func messageMatches(sent, observed string) bool {
	if observed == sent {
		return true
	}
	suffix, ok := strings.CutPrefix(observed, sent+"{")
	if !ok {
		return false
	}
	suffix = strings.TrimSuffix(suffix, "}")
	if suffix == "" {
		return false
	}
	for _, char := range suffix {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func removeID(ids []string, target string) []string {
	for i, id := range ids {
		if id == target {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

func stringID(id int64) string {
	if id == 0 {
		return "0"
	}
	const digits = "0123456789"
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = digits[id%10]
		id /= 10
	}
	return string(buf[i:])
}
