package storage

import (
	"fmt"
	"testing"

	"github.com/mailhog/data"
)

func TestMemoryMaxMessagesDefault(t *testing.T) {
	t.Setenv("MH_MEMORY_MAX_MESSAGES", "")
	memory := CreateInMemory()
	if memory.maxMessages != defaultMemoryMaxMessages {
		t.Fatalf("maxMessages = %d, want %d", memory.maxMessages, defaultMemoryMaxMessages)
	}
}

func TestMemoryMaxMessagesInvalidFallsBack(t *testing.T) {
	for _, raw := range []string{"0", "-3", "nope"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("MH_MEMORY_MAX_MESSAGES", raw)
			memory := CreateInMemory()
			if memory.maxMessages != defaultMemoryMaxMessages {
				t.Fatalf("MH_MEMORY_MAX_MESSAGES=%q maxMessages = %d, want %d", raw, memory.maxMessages, defaultMemoryMaxMessages)
			}
		})
	}
}

func TestStoreDropsOldestBeyondLimit(t *testing.T) {
	t.Setenv("MH_MEMORY_MAX_MESSAGES", "3")
	memory := CreateInMemory()

	for i := 0; i < 5; i++ {
		id := data.MessageID(fmt.Sprintf("m%d", i))
		if _, err := memory.Store(&data.Message{ID: id}); err != nil {
			t.Fatal(err)
		}
	}

	if memory.Count() != 3 {
		t.Fatalf("Count() = %d, want 3", memory.Count())
	}

	for _, id := range []string{"m0", "m1"} {
		msg, err := memory.Load(id)
		if err != nil {
			t.Fatal(err)
		}
		if msg != nil {
			t.Fatalf("Load(%s) = %v, want dropped", id, msg.ID)
		}
	}

	for _, id := range []string{"m2", "m3", "m4"} {
		msg, err := memory.Load(id)
		if err != nil {
			t.Fatal(err)
		}
		if msg == nil || string(msg.ID) != id {
			t.Fatalf("Load(%s) missing", id)
		}
	}

	listed, err := memory.List(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if listed == nil || len(*listed) != 3 {
		t.Fatalf("List len = %d, want 3", len(*listed))
	}
	got := []string{string((*listed)[0].ID), string((*listed)[1].ID), string((*listed)[2].ID)}
	want := []string{"m4", "m3", "m2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List order = %v, want %v", got, want)
		}
	}

	if err := memory.DeleteOne("m3"); err != nil {
		t.Fatal(err)
	}
	if memory.Count() != 2 {
		t.Fatalf("Count() after delete = %d, want 2", memory.Count())
	}
	if _, err := memory.Store(&data.Message{ID: "m5"}); err != nil {
		t.Fatal(err)
	}
	if memory.Count() != 3 {
		t.Fatalf("Count() after refill = %d, want 3", memory.Count())
	}
	if msg, _ := memory.Load("m2"); msg == nil {
		t.Fatal("m2 should stay until the cap is exceeded")
	}
	if _, err := memory.Store(&data.Message{ID: "m6"}); err != nil {
		t.Fatal(err)
	}
	if memory.Count() != 3 {
		t.Fatalf("Count() after overflow = %d, want 3", memory.Count())
	}
	if msg, _ := memory.Load("m2"); msg != nil {
		t.Fatal("oldest remaining message was not dropped after refill")
	}
	if msg, _ := memory.Load("m6"); msg == nil || string(msg.ID) != "m6" {
		t.Fatal("newest message missing after refill")
	}
}
