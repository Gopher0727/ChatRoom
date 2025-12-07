package guild

import (
	"fmt"
	"testing"
)

func TestUnreadCount(t *testing.T) {
	// 1. Create Owner and Guild
	ownerToken := CreateUser(t)
	guildID := CreateGuild(t, ownerToken)
	inviteCode := CreateInvite(t, ownerToken, guildID)

	t.Logf("Created Guild %d with invite code %s", guildID, inviteCode)

	// 2. Create Member and Join
	memberToken := CreateUser(t) // CreateUser generates a random user
	if err := joinGuild(memberToken, inviteCode); err != nil {
		t.Fatalf("Member failed to join guild: %v", err)
	}

	// 3. Owner sends 5 messages
	var lastSeq int64
	for i := 1; i <= 5; i++ {
		msg := sendMessage(t, ownerToken, guildID, fmt.Sprintf("Message %d", i))
		lastSeq = msg.SequenceID
	}
	t.Logf("Sent 5 messages, last sequence id: %d", lastSeq)

	// 4. Member checks unread count (should be 5)
	guilds := getMyGuilds(t, memberToken)
	var found bool
	for _, g := range guilds {
		if g.ID == guildID {
			found = true
			t.Logf("Guild %d unread count: %d", g.ID, g.UnreadCount)
			// Note: Depending on implementation, initial LastReadMsgID might be 0.
			// If messages started at Seq 1, and there are 5 messages, maxSeq=5.
			// Unread = 5 - 0 = 5.
			if g.UnreadCount != 5 {
				t.Errorf("Expected 5 unread messages, got %d", g.UnreadCount)
			}
		}
	}
	if !found {
		t.Fatalf("Member did not find the joined guild in list")
	}

	// 5. Member acks message 3
	ackSeq := int64(3)
	ackMessage(t, memberToken, guildID, ackSeq)
	t.Logf("Acked sequence id %d", ackSeq)

	// 6. Member checks unread count (should be 2)
	guilds = getMyGuilds(t, memberToken)
	for _, g := range guilds {
		if g.ID == guildID {
			t.Logf("Guild %d unread count after ack: %d", g.ID, g.UnreadCount)
			if g.UnreadCount != 2 {
				t.Errorf("Expected 2 unread messages, got %d", g.UnreadCount)
			}
		}
	}
}
