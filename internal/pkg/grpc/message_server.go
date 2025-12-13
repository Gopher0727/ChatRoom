package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Gopher0727/ChatRoom/internal/pkg/proto"
	"github.com/Gopher0727/ChatRoom/internal/service"
)

// MessageServer 实现 MessageService gRPC 服务
type MessageServer struct {
	pb.UnimplementedMessageServiceServer
	messageService service.IMessageService
}

// NewMessageServer 创建新的 Message gRPC 服务器
func NewMessageServer(messageService service.IMessageService) *MessageServer {
	return &MessageServer{
		messageService: messageService,
	}
}

// SendMessage 发送消息（内部调用）
func (s *MessageServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}
	if req.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}

	// 调用业务层发送消息
	message, err := s.messageService.SendMessage(ctx, req.UserId, req.GuildId, req.Content)
	if err != nil {
		return &pb.SendMessageResponse{
			Error: err.Error(),
		}, nil
	}

	// 转换为 protobuf 消息
	wsMessage := &pb.WSMessage{
		MessageId: message.ID,
		UserId:    message.UserID,
		GuildId:   message.GuildID,
		Content:   message.Content,
		SeqId:     message.SeqID,
		Timestamp: message.CreatedAt.Unix(),
		Type:      pb.MessageType_TEXT,
	}

	return &pb.SendMessageResponse{
		Message: wsMessage,
	}, nil
}

// GetHistory 获取历史消息
func (s *MessageServer) GetHistory(ctx context.Context, req *pb.HistoryRequest) (*pb.HistoryResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}

	// 调用业务层获取历史消息
	messages, hasMore, err := s.messageService.GetMessagesWithUser(ctx, req.GuildId, req.LastSeqId, int(req.Limit))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// 转换为 protobuf 消息
	wsMessages := make([]*pb.WSMessage, len(messages))
	for i, msg := range messages {
		wsMessages[i] = &pb.WSMessage{
			MessageId: msg.ID,
			UserId:    msg.UserID,
			GuildId:   msg.GuildID,
			Content:   msg.Content,
			SeqId:     msg.SeqID,
			Timestamp: msg.CreatedAt.Unix(),
			Type:      pb.MessageType_TEXT,
			Username:  msg.Username,
		}
	}

	return &pb.HistoryResponse{
		Messages: wsMessages,
		HasMore:  hasMore,
	}, nil
}

// BatchGetMessages 批量获取消息
func (s *MessageServer) BatchGetMessages(ctx context.Context, req *pb.BatchGetMessagesRequest) (*pb.BatchGetMessagesResponse, error) {
	if len(req.MessageIds) == 0 {
		return &pb.BatchGetMessagesResponse{
			Messages: []*pb.WSMessage{},
		}, nil
	}

	// 调用业务层批量获取消息
	messages, err := s.messageService.BatchGetMessages(ctx, req.MessageIds)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// 转换为 protobuf 消息
	wsMessages := make([]*pb.WSMessage, len(messages))
	for i, msg := range messages {
		wsMessages[i] = &pb.WSMessage{
			MessageId: msg.ID,
			UserId:    msg.UserID,
			GuildId:   msg.GuildID,
			Content:   msg.Content,
			SeqId:     msg.SeqID,
			Timestamp: msg.CreatedAt.Unix(),
			Type:      pb.MessageType_TEXT,
		}
	}

	return &pb.BatchGetMessagesResponse{
		Messages: wsMessages,
	}, nil
}
