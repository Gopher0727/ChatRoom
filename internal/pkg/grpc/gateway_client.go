package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Gopher0727/ChatRoom/internal/pkg/proto"
)

// GatewayClient 封装 Gateway gRPC 客户端
type GatewayClient struct {
	conn   *grpc.ClientConn
	client pb.GatewayServiceClient
}

// NewGatewayClient 创建新的 Gateway gRPC 客户端
func NewGatewayClient(address string) (*GatewayClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}

	return &GatewayClient{
		conn:   conn,
		client: pb.NewGatewayServiceClient(conn),
	}, nil
}

// Close 关闭客户端连接
func (c *GatewayClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// PushMessage 推送消息到指定用户
func (c *GatewayClient) PushMessage(ctx context.Context, userID string, message *pb.WSMessage) error {
	resp, err := c.client.PushMessage(ctx, &pb.PushMessageRequest{
		UserId:  userID,
		Message: message,
	})
	if err != nil {
		return fmt.Errorf("grpc call failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("push failed: %s", resp.Error)
	}

	return nil
}

// BroadcastToGuild 广播消息到 Guild
func (c *GatewayClient) BroadcastToGuild(ctx context.Context, guildID string, message *pb.WSMessage, excludeUserIDs []string) (int32, error) {
	resp, err := c.client.BroadcastToGuild(ctx, &pb.BroadcastRequest{
		GuildId:        guildID,
		Message:        message,
		ExcludeUserIds: excludeUserIDs,
	})
	if err != nil {
		return 0, fmt.Errorf("grpc call failed: %w", err)
	}

	return resp.DeliveredCount, nil
}

// CheckUserOnline 检查用户是否在线
func (c *GatewayClient) CheckUserOnline(ctx context.Context, userID string) (bool, string, error) {
	resp, err := c.client.CheckUserOnline(ctx, &pb.UserStatusRequest{
		UserId: userID,
	})
	if err != nil {
		return false, "", fmt.Errorf("grpc call failed: %w", err)
	}

	return resp.Online, resp.GatewayNode, nil
}

// GetNodeInfo 获取节点信息
func (c *GatewayClient) GetNodeInfo(ctx context.Context) (*pb.NodeInfoResponse, error) {
	resp, err := c.client.GetNodeInfo(ctx, &pb.NodeInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("grpc call failed: %w", err)
	}

	return resp, nil
}

// HealthCheck 健康检查
func (c *GatewayClient) HealthCheck(ctx context.Context) (bool, error) {
	resp, err := c.client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return false, fmt.Errorf("grpc call failed: %w", err)
	}
	return resp.Healthy, nil
}
