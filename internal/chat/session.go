package chat

import "github.com/yunzhu457/CCode/internal/provider"

type Session struct {
	messages []provider.Message
}

func NewSession() *Session {
	return &Session{}
}

func (s *Session) AddUserMessage(content string) {
	s.messages = append(s.messages, provider.Message{Role: provider.RoleUser, Content: content})
}

func (s *Session) AddAssistantMessage(content string) {
	s.messages = append(s.messages, provider.Message{Role: provider.RoleAssistant, Content: content})
}

func (s *Session) Messages() []provider.Message {
	out := make([]provider.Message, len(s.messages))
	copy(out, s.messages)
	return out
}
