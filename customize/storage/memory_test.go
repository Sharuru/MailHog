package storage

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/mailhog/data"
	"golang.org/x/text/encoding/japanese"
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

func TestContainingFindsDecodedBody(t *testing.T) {
	t.Setenv("MH_MEMORY_MAX_MESSAGES", "")
	memory := CreateInMemory()

	plain := &data.Message{
		ID: "plain",
		Content: &data.Content{
			Headers: map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}},
			Body:    "plain token ALPHA-PLAIN",
		},
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("<p>hidden token <b>BETA-HTML</b></p>"))
	html := &data.Message{
		ID: "html",
		Content: &data.Content{
			Headers: map[string][]string{
				"Content-Type": {"multipart/alternative; boundary=bound"},
				"Subject":      {"ordinary"},
			},
			Body: "--bound raw base64 should not be required",
		},
		MIME: &data.MIMEBody{Parts: []*data.Content{
			{
				Headers: map[string][]string{
					"Content-Type":              {"text/html; charset=utf-8"},
					"Content-Transfer-Encoding": {"base64"},
				},
				Body: encoded,
			},
		}},
	}
	qp := &data.Message{
		ID: "qp",
		Content: &data.Content{
			Headers: map[string][]string{
				"Content-Type":              {"text/plain; charset=utf-8"},
				"Content-Transfer-Encoding": {"quoted-printable"},
			},
			Body: "=E5=8F=91=E7=A5=A8=E5=8F=B7 GAMMA-QP",
		},
	}
	shiftJIS, err := japanese.ShiftJIS.NewEncoder().Bytes([]byte("検索DELTA"))
	if err != nil {
		t.Fatal(err)
	}
	sjis := &data.Message{
		ID: "sjis",
		Content: &data.Content{
			Headers: map[string][]string{"Content-Type": {"text/plain; charset=shift_jis"}},
			Body:    string(shiftJIS),
		},
	}
	subjectRaw := base64.StdEncoding.EncodeToString([]byte("主题EPSILON"))
	subjectOnly := &data.Message{
		ID: "subject",
		Content: &data.Content{
			Headers: map[string][]string{
				"Subject":      {"=?UTF-8?B?" + subjectRaw + "?="},
				"Content-Type": {"text/plain; charset=utf-8"},
			},
			Body: "nothing special",
		},
	}
	for _, msg := range []*data.Message{plain, html, qp, sjis, subjectOnly} {
		if _, err := memory.Store(msg); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		query string
		want  string
	}{
		{"alpha-plain", "plain"},
		{"beta-html", "html"},
		{"发票号", "qp"},
		{"gamma-qp", "qp"},
		{"検索delta", "sjis"},
		{"主题epsilon", "subject"},
		{"not-in-any-mail", ""},
	}
	for _, tc := range cases {
		msgs, total, err := memory.Search("containing", tc.query, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if tc.want == "" {
			if total != 0 {
				t.Fatalf("query %q total = %d, want 0", tc.query, total)
			}
			continue
		}
		if total != 1 || msgs == nil || len(*msgs) != 1 || string((*msgs)[0].ID) != tc.want {
			t.Fatalf("query %q got total %d id %v, want %s", tc.query, total, idsOf(msgs), tc.want)
		}
	}
}

func TestFromToFindsDecodedDisplayNames(t *testing.T) {
	t.Setenv("MH_MEMORY_MAX_MESSAGES", "")
	memory := CreateInMemory()
	name := base64.StdEncoding.EncodeToString([]byte("测试收件人"))
	from := base64.StdEncoding.EncodeToString([]byte("测试发件人"))
	msg := &data.Message{
		ID:   "named",
		From: &data.Path{Mailbox: "sender", Domain: "example.com"},
		To:   []*data.Path{{Mailbox: "recv", Domain: "example.com"}},
		Content: &data.Content{
			Headers: map[string][]string{
				"From": {"=?UTF-8?B?" + from + "?= <sender@example.com>"},
				"To":   {"=?UTF-8?B?" + name + "?= <recv@example.com>"},
				"Cc":   {"=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("抄送人")) + "?= <cc@example.com>"},
			},
			Body: "body",
		},
	}
	if _, err := memory.Store(msg); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		kind, query string
	}{
		{"to", "测试收件人"},
		{"to", "抄送人"},
		{"from", "测试发件人"},
	} {
		_, total, err := memory.Search(tc.kind, tc.query, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 {
			t.Fatalf("%s %q total = %d, want 1", tc.kind, tc.query, total)
		}
	}
}

func idsOf(msgs *data.Messages) []string {
	if msgs == nil {
		return nil
	}
	out := make([]string, len(*msgs))
	for i, m := range *msgs {
		out[i] = string(m.ID)
	}
	return out
}
