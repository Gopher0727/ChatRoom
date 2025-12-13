package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Gopher0727/ChatRoom/internal/pkg/proto"
	"github.com/Gopher0727/ChatRoom/internal/repository"
)

// UserServer 实现 UserService gRPC 服务
type UserServer struct {
	pb.UnimplementedUserServiceServer
	userRepo repository.UserRepository
}

// NewUserServer 创建新的 User gRPC 服务器
func NewUserServer(userRepo repository.UserRepository) *UserServer {
	return &UserServer{
		userRepo: userRepo,
	}
}

// GetUser 获取用户信息
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	user, err := s.userRepo.FindByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return &pb.GetUserResponse{
		UserId:    user.ID,
		Username:  user.UserName,
		HubId:     user.HubID,
		CreatedAt: user.CreatedAt.Unix(),
	}, nil
}

// BatchGetUsers 批量获取用户信息
func (s *UserServer) BatchGetUsers(ctx context.Context, req *pb.BatchGetUsersRequest) (*pb.BatchGetUsersResponse, error) {
	if len(req.UserIds) == 0 {
		return &pb.BatchGetUsersResponse{
			Users: []*pb.GetUserResponse{},
		}, nil
	}

	users := make([]*pb.GetUserResponse, 0, len(req.UserIds))
	for _, userID := range req.UserIds {
		user, err := s.userRepo.FindByID(ctx, userID)
		if err != nil {
			// 跳过不存在的用户
			continue
		}

		users = append(users, &pb.GetUserResponse{
			UserId:    user.ID,
			Username:  user.UserName,
			HubId:     user.HubID,
			CreatedAt: user.CreatedAt.Unix(),
		})
	}

	return &pb.BatchGetUsersResponse{
		Users: users,
	}, nil
}

// UpdateUserStatus 更新用户在线状态
func (s *UserServer) UpdateUserStatus(ctx context.Context, req *pb.UpdateUserStatusRequest) (*pb.UpdateUserStatusResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// 这里可以更新 Redis 中的用户在线状态
	// 暂时返回成功
	// TODO
	return &pb.UpdateUserStatusResponse{
		Success: true,
	}, nil
}
