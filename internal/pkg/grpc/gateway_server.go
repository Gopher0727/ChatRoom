package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/Gopher0727/ChatRoom/internal/pkg/gateway"
	pb "github.com/Gopher0727/ChatRoom/internal/pkg/proto"
)

// GatewayServer 实现 GatewayService gRPC 服务
type GatewayServer struct {
	pb.UnimplementedGatewayServiceServer
	manager   *gateway.ConnectionManager
	nodeID    string
	address   string
	startTime time.Time
}

// NewGatewayServer 创建新的 Gateway gRPC 服务器
func NewGatewayServer(manager *gateway.ConnectionManager, nodeID, address string) *GatewayServer {
	return &GatewayServer{
		manager:   manager,
		nodeID:    nodeID,
		address:   address,
		startTime: time.Now(),
	}
}

// PushMessage 推送消息到指定用户的连接
func (s *GatewayServer) PushMessage(ctx context.Context, req *pb.PushMessageRequest) (*pb.PushMessageResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.Message == nil {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}

	// 获取用户连接
	conn, exists := s.manager.GetConnection(req.UserId)
	if !exists {
		return &pb.PushMessageResponse{
			Success: false,
			Error:   "user not connected to this gateway",
		}, nil
	}

	// 序列化消息
	data, err := proto.Marshal(req.Message)
	if err != nil {
		return &pb.PushMessageResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal message: %v", err),
		}, nil
	}

	// 推送消息
	err = conn.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return &pb.PushMessageResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to push message: %v", err),
		}, nil
	}

	return &pb.PushMessageResponse{
		Success: true,
	}, nil
}

// BroadcastToGuild 推送消息到 Guild 的所有在线成员
func (s *GatewayServer) BroadcastToGuild(ctx context.Context, req *pb.BroadcastRequest) (*pb.BroadcastResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}
	if req.Message == nil {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}

	excludeMap := make(map[string]bool)
	for _, uid := range req.ExcludeUserIds {
		excludeMap[uid] = true
	}

	deliveredCount := int32(0)
	var failedUserIDs []string

	// 序列化消息
	data, err := proto.Marshal(req.Message)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to marshal message: %v", err))
	}

	// 遍历所有连接，找到属于该 Guild 的用户
	allConns := s.manager.GetAllConnections()
	for userID, conn := range allConns {
		// 跳过排除的用户
		if excludeMap[userID] {
			continue
		}

		// 推送消息
		err := conn.WriteMessage(websocket.BinaryMessage, data)
		if err != nil {
			failedUserIDs = append(failedUserIDs, userID)
		} else {
			deliveredCount++
		}
	}

	return &pb.BroadcastResponse{
		DeliveredCount: deliveredCount,
		FailedUserIds:  failedUserIDs,
	}, nil
}

// CheckUserOnline 检查用户是否在线
func (s *GatewayServer) CheckUserOnline(ctx context.Context, req *pb.UserStatusRequest) (*pb.UserStatusResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	_, exists := s.manager.GetConnection(req.UserId)

	return &pb.UserStatusResponse{
		Online:      exists,
		GatewayNode: s.address,
	}, nil
}

// GetNodeInfo 获取 Gateway 节点信息
func (s *GatewayServer) GetNodeInfo(ctx context.Context, req *pb.NodeInfoRequest) (*pb.NodeInfoResponse, error) {
	allConns := s.manager.GetAllConnections()
	uptime := time.Since(s.startTime)

	return &pb.NodeInfoResponse{
		NodeId:          s.nodeID,
		Address:         s.address,
		ConnectionCount: int32(len(allConns)),
		UptimeSeconds:   int64(uptime.Seconds()),
	}, nil
}

// HealthCheck 健康检查
func (s *GatewayServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Healthy: true,
		Status:  "ok",
	}, nil
}
