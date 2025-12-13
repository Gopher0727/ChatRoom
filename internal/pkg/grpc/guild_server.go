package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Gopher0727/ChatRoom/internal/pkg/proto"
	"github.com/Gopher0727/ChatRoom/internal/repository"
)

// GuildServer 实现 GuildService gRPC 服务
type GuildServer struct {
	pb.UnimplementedGuildServiceServer
	guildRepo repository.GuildRepository
}

// NewGuildServer 创建新的 Guild gRPC 服务器
func NewGuildServer(guildRepo repository.GuildRepository) *GuildServer {
	return &GuildServer{
		guildRepo: guildRepo,
	}
}

// GetGuild 获取 Guild 信息
func (s *GuildServer) GetGuild(ctx context.Context, req *pb.GetGuildRequest) (*pb.GetGuildResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}

	guild, err := s.guildRepo.FindByID(ctx, req.GuildId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "guild not found")
	}

	return &pb.GetGuildResponse{
		GuildId:    guild.ID,
		Name:       guild.Name,
		OwnerId:    guild.OwnerID,
		InviteCode: guild.InviteCode,
		CreatedAt:  guild.CreatedAt.Unix(),
	}, nil
}

// GetGuildMembers 获取 Guild 成员列表
func (s *GuildServer) GetGuildMembers(ctx context.Context, req *pb.GetGuildMembersRequest) (*pb.GetGuildMembersResponse, error) {
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}

	// 获取 Guild 成员
	members, err := s.guildRepo.GetMembers(ctx, req.GuildId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// 提取用户ID
	userIDs := make([]string, len(members))
	for i, member := range members {
		userIDs[i] = member.UserID
	}

	return &pb.GetGuildMembersResponse{
		UserIds:    userIDs,
		TotalCount: int32(len(userIDs)),
	}, nil
}

// CheckMembership 检查用户是否是 Guild 成员
func (s *GuildServer) CheckMembership(ctx context.Context, req *pb.CheckMembershipRequest) (*pb.CheckMembershipResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}

	isMember, err := s.guildRepo.IsMember(ctx, req.GuildId, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	response := &pb.CheckMembershipResponse{
		IsMember: isMember,
	}

	if isMember {
		// 可以获取加入时间
		// TODO: 从数据库获取实际的加入时间
		response.JoinedAt = 0
	}

	return response, nil
}
