package storage

import (
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/mailhog/data"
)

const defaultMemoryMaxMessages = 5000

// InMemory is an in memory storage backend
type InMemory struct {
	MessageIDIndex map[string]int
	Messages       []*data.Message
	maxMessages    int
	mu             sync.Mutex
}

// CreateInMemory creates a new in memory storage backend.
// MH_MEMORY_MAX_MESSAGES overrides how many messages are kept (default 5000).
// Messages beyond that are dropped oldest-first. The store is still empty after a restart.
func CreateInMemory() *InMemory {
	maxMessages := memoryMaxMessages()
	log.Printf("In-memory storage will keep the most recent %d messages", maxMessages)
	return &InMemory{
		MessageIDIndex: make(map[string]int),
		Messages:       make([]*data.Message, 0),
		maxMessages:    maxMessages,
	}
}

func memoryMaxMessages() int {
	raw := strings.TrimSpace(os.Getenv("MH_MEMORY_MAX_MESSAGES"))
	if raw == "" {
		return defaultMemoryMaxMessages
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("Invalid MH_MEMORY_MAX_MESSAGES %q, using %d", raw, defaultMemoryMaxMessages)
		return defaultMemoryMaxMessages
	}
	return n
}

// Store stores a message and returns its storage ID.
// When the configured cap is exceeded, the oldest messages are dropped.
func (memory *InMemory) Store(m *data.Message) (string, error) {
	memory.mu.Lock()
	defer memory.mu.Unlock()
	memory.Messages = append(memory.Messages, m)
	memory.MessageIDIndex[string(m.ID)] = len(memory.Messages) - 1
	memory.trimLocked()
	return string(m.ID), nil
}

func (memory *InMemory) trimLocked() {
	if memory.maxMessages <= 0 || len(memory.Messages) <= memory.maxMessages {
		return
	}
	memory.Messages = memory.Messages[len(memory.Messages)-memory.maxMessages:]
	memory.MessageIDIndex = make(map[string]int, len(memory.Messages))
	for i, m := range memory.Messages {
		memory.MessageIDIndex[string(m.ID)] = i
	}
}

// Count returns the number of stored messages
func (memory *InMemory) Count() int {
	return len(memory.Messages)
}

// Search finds messages matching the query
func (memory *InMemory) Search(kind, query string, start, limit int) (*data.Messages, int, error) {
	// FIXME needs optimising, or replacing with a proper db!
	query = strings.ToLower(query)
	var filteredMessages = make([]*data.Message, 0)
	for _, m := range memory.Messages {
		doAppend := false

		switch kind {
		case "to":
			for _, to := range m.To {
				if strings.Contains(strings.ToLower(to.Mailbox+"@"+to.Domain), query) {
					doAppend = true
					break
				}
			}
			if !doAppend && m.Content != nil {
				for _, key := range []string{"To", "Cc", "Bcc"} {
					if headerFieldContains(m.Content.Headers, key, query) {
						doAppend = true
						break
					}
				}
			}
		case "from":
			if strings.Contains(strings.ToLower(m.From.Mailbox+"@"+m.From.Domain), query) {
				doAppend = true
			}
			if !doAppend && m.Content != nil && headerFieldContains(m.Content.Headers, "From", query) {
				doAppend = true
			}
		case "containing":
			if messageMatchesQuery(m, query) {
				doAppend = true
			}
		}

		if doAppend {
			filteredMessages = append(filteredMessages, m)
		}
	}

	var messages = make([]data.Message, 0)

	if len(filteredMessages) == 0 || start > len(filteredMessages) {
		msgs := data.Messages(messages)
		return &msgs, 0, nil
	}

	if start+limit > len(filteredMessages) {
		limit = len(filteredMessages) - start
	}

	start = len(filteredMessages) - start - 1
	end := start - limit

	if start < 0 {
		start = 0
	}
	if end < -1 {
		end = -1
	}

	for i := start; i > end; i-- {
		//for _, m := range memory.MessageIndex[start:end] {
		messages = append(messages, *filteredMessages[i])
	}

	msgs := data.Messages(messages)
	return &msgs, len(filteredMessages), nil
}

// List lists stored messages by index
func (memory *InMemory) List(start int, limit int) (*data.Messages, error) {
	var messages = make([]data.Message, 0)

	if len(memory.Messages) == 0 || start > len(memory.Messages) {
		msgs := data.Messages(messages)
		return &msgs, nil
	}

	if start+limit > len(memory.Messages) {
		limit = len(memory.Messages) - start
	}

	start = len(memory.Messages) - start - 1
	end := start - limit

	if start < 0 {
		start = 0
	}
	if end < -1 {
		end = -1
	}

	for i := start; i > end; i-- {
		//for _, m := range memory.MessageIndex[start:end] {
		messages = append(messages, *memory.Messages[i])
	}

	msgs := data.Messages(messages)
	return &msgs, nil
}

// DeleteOne deletes an individual message by storage ID
func (memory *InMemory) DeleteOne(id string) error {
	memory.mu.Lock()
	defer memory.mu.Unlock()

	var index int
	var ok bool

	if index, ok = memory.MessageIDIndex[id]; !ok && true {
		return errors.New("message not found")
	}

	delete(memory.MessageIDIndex, id)
	for k, v := range memory.MessageIDIndex {
		if v > index {
			memory.MessageIDIndex[k] = v - 1
		}
	}
	memory.Messages = append(memory.Messages[:index], memory.Messages[index+1:]...)
	return nil
}

// DeleteAll deletes all in memory messages
func (memory *InMemory) DeleteAll() error {
	memory.mu.Lock()
	defer memory.mu.Unlock()
	memory.Messages = make([]*data.Message, 0)
	memory.MessageIDIndex = make(map[string]int)
	return nil
}

// Load returns an individual message by storage ID
func (memory *InMemory) Load(id string) (*data.Message, error) {
	if idx, ok := memory.MessageIDIndex[id]; ok {
		return memory.Messages[idx], nil
	}
	return nil, nil
}
