package chat

import (
	"testing"

	"github.com/yunzhu457/CCode/internal/provider"
)

func TestSessionStoresConversationTurns(t *testing.T) {
	session := NewSession()

	session.AddUserMessage("hello")
	session.AddAssistantMessage("hi")
	session.AddUserMessage("what did I say?")

	got := session.Messages()
	want := []provider.Message{
		{Role: provider.RoleUser, Content: "hello"},
		{Role: provider.RoleAssistant, Content: "hi"},
		{Role: provider.RoleUser, Content: "what did I say?"},
	}

	if len(got) != len(want) {
		t.Fatalf("len(Messages()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Messages()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	got[0].Content = "mutated"
	if session.Messages()[0].Content == "mutated" {
		t.Fatal("Messages() returned mutable internal slice")
	}
}
