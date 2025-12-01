package ai

import (
	"sync"
	"time"

	"eulerguard/pkg/types"
)

type Conversation struct {
	ID        string    `json:"id"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ConversationStore struct {
	mu      sync.RWMutex
	convos  map[string]*Conversation
	maxAge  time.Duration
	maxMsgs int
}

func NewConversationStore() *ConversationStore {
	store := &ConversationStore{
		convos:  make(map[string]*Conversation),
		maxAge:  30 * time.Minute,
		maxMsgs: 20,
	}
	go store.cleanupLoop()
	return store
}

func (s *ConversationStore) GetOrCreate(sessionID string) *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conv, ok := s.convos[sessionID]; ok {
		conv.UpdatedAt = time.Now()
		return conv
	}

	conv := &Conversation{
		ID:        sessionID,
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.convos[sessionID] = conv
	return conv
}

func (s *ConversationStore) AddMessage(sessionID string, msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.convos[sessionID]
	if !ok {
		return
	}

	conv.Messages = append(conv.Messages, msg)
	conv.UpdatedAt = time.Now()
	if len(conv.Messages) > s.maxMsgs {
		conv.Messages = conv.Messages[len(conv.Messages)-s.maxMsgs:]
	}
}

func (s *ConversationStore) GetMessages(sessionID string) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, ok := s.convos[sessionID]
	if !ok {
		return nil
	}

	result := make([]Message, len(conv.Messages))
	copy(result, conv.Messages)
	return result
}

func (s *ConversationStore) Clear(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.convos, sessionID)
}

func (s *ConversationStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, conv := range s.convos {
			if now.Sub(conv.UpdatedAt) > s.maxAge {
				delete(s.convos, id)
			}
		}
		s.mu.Unlock()
	}
}

func BuildChatMessages(history []Message, snapshot types.SystemSnapshot, userMessage string) []Message {
	messages := make([]Message, 0, len(history)+3)

	messages = append(messages, Message{
		Role:    "system",
		Content: ChatSystemPrompt,
	})

	contextMsg := FormatContextForChat(snapshot)
	messages = append(messages, Message{
		Role:    "system",
		Content: contextMsg,
	})

	for _, msg := range history {
		messages = append(messages, Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	messages = append(messages, Message{
		Role:    "user",
		Content: userMessage,
	})

	return messages
}
