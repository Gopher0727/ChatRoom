package guild

import (
	"fmt"
	"testing"
)

func TestMessageSequence(t *testing.T) {
	token := CreateUser(t)
	guildID := CreateGuild(t, token)

	t.Logf("Created Guild ID: %d", guildID)

	// 发送 3 条消息
	var seqs []int64
	for i := 1; i <= 3; i++ {
		content := fmt.Sprintf("Message %d", i)
		msg := sendMessage(t, token, guildID, content)
		t.Logf("Sent message: %s, Seq: %d", content, msg.SequenceID)

		if msg.SequenceID <= 0 {
			t.Errorf("Expected positive SequenceID, got %d", msg.SequenceID)
		}
		if len(seqs) > 0 && msg.SequenceID <= seqs[len(seqs)-1] {
			t.Errorf("SequenceID should be increasing. Prev: %d, Curr: %d", seqs[len(seqs)-1], msg.SequenceID)
		}
		seqs = append(seqs, msg.SequenceID)
	}

	// 测试增量拉取 (GetMessagesAfterSequence)
	// 拉取第 1 条消息之后的所有消息 (应该返回第 2 和 第 3 条)
	afterSeq := seqs[0]
	messages := getMessagesAfter(t, token, guildID, afterSeq)

	t.Logf("Requested messages after seq %d, got %d messages", afterSeq, len(messages))

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	if len(messages) > 0 {
		if messages[0].SequenceID != seqs[1] {
			t.Errorf("Expected first message seq to be %d, got %d", seqs[1], messages[0].SequenceID)
		}
		if messages[1].SequenceID != seqs[2] {
			t.Errorf("Expected second message seq to be %d, got %d", seqs[2], messages[1].SequenceID)
		}
	}
}
