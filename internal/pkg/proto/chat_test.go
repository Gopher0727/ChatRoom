package proto

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"google.golang.org/protobuf/proto"
)

func TestWSMessage_MarshalUnmarshal(t *testing.T) {
	original := &WSMessage{
		MessageId: "msg_123",
		UserId:    "user_456",
		GuildId:   "guild_789",
		Content:   "Hello, World!",
		SeqId:     100,
		Timestamp: 1234567890,
		Type:      MessageType_TEXT,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &WSMessage{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify
	if decoded.MessageId != original.MessageId {
		t.Errorf("MessageId mismatch: got %v, want %v", decoded.MessageId, original.MessageId)
	}
	if decoded.UserId != original.UserId {
		t.Errorf("UserId mismatch: got %v, want %v", decoded.UserId, original.UserId)
	}
	if decoded.GuildId != original.GuildId {
		t.Errorf("GuildId mismatch: got %v, want %v", decoded.GuildId, original.GuildId)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content mismatch: got %v, want %v", decoded.Content, original.Content)
	}
	if decoded.SeqId != original.SeqId {
		t.Errorf("SeqId mismatch: got %v, want %v", decoded.SeqId, original.SeqId)
	}
	if decoded.Timestamp != original.Timestamp {
		t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp, original.Timestamp)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: got %v, want %v", decoded.Type, original.Type)
	}
}

func TestHistoryRequest_MarshalUnmarshal(t *testing.T) {
	original := &HistoryRequest{
		GuildId:   "guild_123",
		LastSeqId: 50,
		Limit:     100,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &HistoryRequest{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify
	if decoded.GuildId != original.GuildId {
		t.Errorf("GuildId mismatch: got %v, want %v", decoded.GuildId, original.GuildId)
	}
	if decoded.LastSeqId != original.LastSeqId {
		t.Errorf("LastSeqId mismatch: got %v, want %v", decoded.LastSeqId, original.LastSeqId)
	}
	if decoded.Limit != original.Limit {
		t.Errorf("Limit mismatch: got %v, want %v", decoded.Limit, original.Limit)
	}
}

func TestHistoryResponse_MarshalUnmarshal(t *testing.T) {
	original := &HistoryResponse{
		Messages: []*WSMessage{
			{
				MessageId: "msg_1",
				UserId:    "user_1",
				GuildId:   "guild_1",
				Content:   "Message 1",
				SeqId:     1,
				Timestamp: 1000,
				Type:      MessageType_TEXT,
			},
			{
				MessageId: "msg_2",
				UserId:    "user_2",
				GuildId:   "guild_1",
				Content:   "Message 2",
				SeqId:     2,
				Timestamp: 2000,
				Type:      MessageType_SYSTEM,
			},
		},
		HasMore: true,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &HistoryResponse{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify
	if len(decoded.Messages) != len(original.Messages) {
		t.Errorf("Messages length mismatch: got %v, want %v", len(decoded.Messages), len(original.Messages))
	}
	if decoded.HasMore != original.HasMore {
		t.Errorf("HasMore mismatch: got %v, want %v", decoded.HasMore, original.HasMore)
	}

	// Verify first message
	if len(decoded.Messages) > 0 {
		if decoded.Messages[0].MessageId != original.Messages[0].MessageId {
			t.Errorf("First message ID mismatch: got %v, want %v", decoded.Messages[0].MessageId, original.Messages[0].MessageId)
		}
		if decoded.Messages[0].Content != original.Messages[0].Content {
			t.Errorf("First message content mismatch: got %v, want %v", decoded.Messages[0].Content, original.Messages[0].Content)
		}
	}
}

func TestMessageType_Enum(t *testing.T) {
	// Test TEXT type
	if MessageType_TEXT.String() != "TEXT" {
		t.Errorf("TEXT string mismatch: got %v, want TEXT", MessageType_TEXT.String())
	}

	// Test SYSTEM type
	if MessageType_SYSTEM.String() != "SYSTEM" {
		t.Errorf("SYSTEM string mismatch: got %v, want SYSTEM", MessageType_SYSTEM.String())
	}

	// Test enum values
	if MessageType_TEXT != 0 {
		t.Errorf("TEXT value mismatch: got %v, want 0", MessageType_TEXT)
	}
	if MessageType_SYSTEM != 1 {
		t.Errorf("SYSTEM value mismatch: got %v, want 1", MessageType_SYSTEM)
	}
}

// genWSMessage generates random WSMessage instances for property testing
func genWSMessage() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),              // MessageId
		gen.Identifier(),              // UserId
		gen.Identifier(),              // GuildId
		gen.AnyString(),               // Content
		gen.Int64Range(0, 1000000),    // SeqId
		gen.Int64Range(0, 9999999999), // Timestamp
		gen.OneConstOf(MessageType_TEXT, MessageType_SYSTEM), // Type
	).Map(func(values []any) *WSMessage {
		return &WSMessage{
			MessageId: values[0].(string),
			UserId:    values[1].(string),
			GuildId:   values[2].(string),
			Content:   values[3].(string),
			SeqId:     values[4].(int64),
			Timestamp: values[5].(int64),
			Type:      values[6].(MessageType),
		}
	})
}

// genHistoryRequest generates random HistoryRequest instances for property testing
func genHistoryRequest() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),           // GuildId
		gen.Int64Range(0, 1000000), // LastSeqId
		gen.Int32Range(1, 1000),    // Limit
	).Map(func(values []any) *HistoryRequest {
		return &HistoryRequest{
			GuildId:   values[0].(string),
			LastSeqId: values[1].(int64),
			Limit:     values[2].(int32),
		}
	})
}

// genHistoryResponse generates random HistoryResponse instances for property testing
func genHistoryResponse() gopter.Gen {
	return gopter.CombineGens(
		gen.SliceOf(genWSMessage()),
		gen.Bool(),
	).Map(func(values []any) *HistoryResponse {
		return &HistoryResponse{
			Messages: values[0].([]*WSMessage),
			HasMore:  values[1].(bool),
		}
	})
}

// Feature: distributed-chat-system, Property 12: Protobuf 序列化往返一致性
// TestProperty_ProtobufRoundTrip tests that any message object can be serialized and deserialized
// preserving all field values
func TestProperty_ProtobufRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Property 1: WSMessage round-trip consistency
	properties.Property("for any WSMessage, serializing then deserializing should preserve all fields",
		prop.ForAll(
			func(original *WSMessage) bool {
				// Serialize
				data, err := proto.Marshal(original)
				if err != nil {
					t.Logf("Failed to marshal: %v", err)
					return false
				}

				// Deserialize
				decoded := &WSMessage{}
				err = proto.Unmarshal(data, decoded)
				if err != nil {
					t.Logf("Failed to unmarshal: %v", err)
					return false
				}

				// Verify all fields are preserved
				return decoded.MessageId == original.MessageId &&
					decoded.UserId == original.UserId &&
					decoded.GuildId == original.GuildId &&
					decoded.Content == original.Content &&
					decoded.SeqId == original.SeqId &&
					decoded.Timestamp == original.Timestamp &&
					decoded.Type == original.Type
			},
			genWSMessage(),
		))

	// Property 2: HistoryRequest round-trip consistency
	properties.Property("for any HistoryRequest, serializing then deserializing should preserve all fields",
		prop.ForAll(
			func(original *HistoryRequest) bool {
				// Serialize
				data, err := proto.Marshal(original)
				if err != nil {
					t.Logf("Failed to marshal: %v", err)
					return false
				}

				// Deserialize
				decoded := &HistoryRequest{}
				err = proto.Unmarshal(data, decoded)
				if err != nil {
					t.Logf("Failed to unmarshal: %v", err)
					return false
				}

				// Verify all fields are preserved
				return decoded.GuildId == original.GuildId &&
					decoded.LastSeqId == original.LastSeqId &&
					decoded.Limit == original.Limit
			},
			genHistoryRequest(),
		))

	// Property 3: HistoryResponse round-trip consistency
	properties.Property("for any HistoryResponse, serializing then deserializing should preserve all fields",
		prop.ForAll(
			func(original *HistoryResponse) bool {
				// Serialize
				data, err := proto.Marshal(original)
				if err != nil {
					t.Logf("Failed to marshal: %v", err)
					return false
				}

				// Deserialize
				decoded := &HistoryResponse{}
				err = proto.Unmarshal(data, decoded)
				if err != nil {
					t.Logf("Failed to unmarshal: %v", err)
					return false
				}

				// Verify HasMore field
				if decoded.HasMore != original.HasMore {
					return false
				}

				// Verify messages length
				if len(decoded.Messages) != len(original.Messages) {
					return false
				}

				// Verify each message
				for i := range original.Messages {
					if decoded.Messages[i].MessageId != original.Messages[i].MessageId ||
						decoded.Messages[i].UserId != original.Messages[i].UserId ||
						decoded.Messages[i].GuildId != original.Messages[i].GuildId ||
						decoded.Messages[i].Content != original.Messages[i].Content ||
						decoded.Messages[i].SeqId != original.Messages[i].SeqId ||
						decoded.Messages[i].Timestamp != original.Messages[i].Timestamp ||
						decoded.Messages[i].Type != original.Messages[i].Type {
						return false
					}
				}

				return true
			},
			genHistoryResponse(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestWSMessage_EmptyFields tests that empty fields are handled correctly
func TestWSMessage_EmptyFields(t *testing.T) {
	// Create message with empty fields
	original := &WSMessage{
		MessageId: "",
		UserId:    "",
		GuildId:   "",
		Content:   "",
		SeqId:     0,
		Timestamp: 0,
		Type:      MessageType_TEXT,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &WSMessage{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify empty strings are preserved
	if decoded.MessageId != "" {
		t.Errorf("MessageId should be empty, got %v", decoded.MessageId)
	}
	if decoded.UserId != "" {
		t.Errorf("UserId should be empty, got %v", decoded.UserId)
	}
	if decoded.GuildId != "" {
		t.Errorf("GuildId should be empty, got %v", decoded.GuildId)
	}
	if decoded.Content != "" {
		t.Errorf("Content should be empty, got %v", decoded.Content)
	}
	if decoded.SeqId != 0 {
		t.Errorf("SeqId should be 0, got %v", decoded.SeqId)
	}
	if decoded.Timestamp != 0 {
		t.Errorf("Timestamp should be 0, got %v", decoded.Timestamp)
	}
}

// TestWSMessage_DefaultValues tests that default values are correctly applied
func TestWSMessage_DefaultValues(t *testing.T) {
	// Create message without setting Type (should default to TEXT)
	msg := &WSMessage{
		MessageId: "msg_123",
		UserId:    "user_456",
		GuildId:   "guild_789",
		Content:   "Test message",
	}

	// Type should default to TEXT (0)
	if msg.Type != MessageType_TEXT {
		t.Errorf("Type should default to TEXT, got %v", msg.Type)
	}

	// Marshal and unmarshal
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	decoded := &WSMessage{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify default type is preserved
	if decoded.Type != MessageType_TEXT {
		t.Errorf("Decoded type should be TEXT, got %v", decoded.Type)
	}
}

// TestWSMessage_SystemType tests SYSTEM message type
func TestWSMessage_SystemType(t *testing.T) {
	original := &WSMessage{
		MessageId: "sys_msg_123",
		UserId:    "system",
		GuildId:   "guild_789",
		Content:   "User joined the guild",
		SeqId:     100,
		Timestamp: 1234567890,
		Type:      MessageType_SYSTEM,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &WSMessage{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify SYSTEM type is preserved
	if decoded.Type != MessageType_SYSTEM {
		t.Errorf("Type should be SYSTEM, got %v", decoded.Type)
	}
}

// TestHistoryRequest_EmptyFields tests that empty fields are handled correctly
func TestHistoryRequest_EmptyFields(t *testing.T) {
	original := &HistoryRequest{
		GuildId:   "",
		LastSeqId: 0,
		Limit:     0,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &HistoryRequest{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify empty/zero values are preserved
	if decoded.GuildId != "" {
		t.Errorf("GuildId should be empty, got %v", decoded.GuildId)
	}
	if decoded.LastSeqId != 0 {
		t.Errorf("LastSeqId should be 0, got %v", decoded.LastSeqId)
	}
	if decoded.Limit != 0 {
		t.Errorf("Limit should be 0, got %v", decoded.Limit)
	}
}

// TestHistoryResponse_EmptyMessages tests that empty message list is handled correctly
func TestHistoryResponse_EmptyMessages(t *testing.T) {
	original := &HistoryResponse{
		Messages: []*WSMessage{},
		HasMore:  false,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &HistoryResponse{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify empty messages list (protobuf treats empty slices as nil after unmarshal)
	if len(decoded.Messages) != 0 {
		t.Errorf("Messages should be empty, got length %v", len(decoded.Messages))
	}
	if decoded.HasMore != false {
		t.Errorf("HasMore should be false, got %v", decoded.HasMore)
	}
}

// TestHistoryResponse_NilMessages tests that nil message list is handled correctly
func TestHistoryResponse_NilMessages(t *testing.T) {
	original := &HistoryResponse{
		Messages: nil,
		HasMore:  true,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &HistoryResponse{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify nil is handled (protobuf treats nil slices as nil, which is safe to use)
	if len(decoded.Messages) != 0 {
		t.Errorf("Messages should be empty, got length %v", len(decoded.Messages))
	}
	if decoded.HasMore != true {
		t.Errorf("HasMore should be true, got %v", decoded.HasMore)
	}
}

// TestWSMessage_LargeContent tests that large content is handled correctly
func TestWSMessage_LargeContent(t *testing.T) {
	// Create a large content string (2000 characters as per spec)
	largeContent := ""
	for i := 0; i < 2000; i++ {
		largeContent += "a"
	}

	original := &WSMessage{
		MessageId: "msg_large",
		UserId:    "user_123",
		GuildId:   "guild_456",
		Content:   largeContent,
		SeqId:     1,
		Timestamp: 1234567890,
		Type:      MessageType_TEXT,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &WSMessage{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify large content is preserved
	if decoded.Content != original.Content {
		t.Errorf("Content mismatch: got length %v, want length %v", len(decoded.Content), len(original.Content))
	}
	if len(decoded.Content) != 2000 {
		t.Errorf("Content length should be 2000, got %v", len(decoded.Content))
	}
}

// TestWSMessage_SpecialCharacters tests that special characters are handled correctly
func TestWSMessage_SpecialCharacters(t *testing.T) {
	original := &WSMessage{
		MessageId: "msg_special",
		UserId:    "user_123",
		GuildId:   "guild_456",
		Content:   "Hello 世界! 🎉 Special chars: <>&\"'",
		SeqId:     1,
		Timestamp: 1234567890,
		Type:      MessageType_TEXT,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &WSMessage{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify special characters are preserved
	if decoded.Content != original.Content {
		t.Errorf("Content mismatch: got %v, want %v", decoded.Content, original.Content)
	}
}

// TestHistoryResponse_MultipleMessages tests handling of multiple messages
func TestHistoryResponse_MultipleMessages(t *testing.T) {
	messages := make([]*WSMessage, 100)
	for i := 0; i < 100; i++ {
		messages[i] = &WSMessage{
			MessageId: "msg_" + string(rune(i)),
			UserId:    "user_1",
			GuildId:   "guild_1",
			Content:   "Message content",
			SeqId:     int64(i + 1),
			Timestamp: int64(1000 + i),
			Type:      MessageType_TEXT,
		}
	}

	original := &HistoryResponse{
		Messages: messages,
		HasMore:  true,
	}

	// Marshal
	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	decoded := &HistoryResponse{}
	err = proto.Unmarshal(data, decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify all messages are preserved
	if len(decoded.Messages) != 100 {
		t.Errorf("Messages length should be 100, got %v", len(decoded.Messages))
	}
	if decoded.HasMore != true {
		t.Errorf("HasMore should be true, got %v", decoded.HasMore)
	}

	// Verify first and last message
	if decoded.Messages[0].SeqId != 1 {
		t.Errorf("First message SeqId should be 1, got %v", decoded.Messages[0].SeqId)
	}
	if decoded.Messages[99].SeqId != 100 {
		t.Errorf("Last message SeqId should be 100, got %v", decoded.Messages[99].SeqId)
	}
}
