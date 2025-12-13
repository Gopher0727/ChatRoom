package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	server   *grpc.Server
	listener net.Listener
	address  string
}

func NewServer(address string) (*Server, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(unaryLoggingInterceptor),   // 一元 RPC 日志拦截器
		grpc.StreamInterceptor(streamLoggingInterceptor), // 流式 RPC 日志拦截器
	)
	return &Server{server: s, listener: listener, address: address}, nil
}

func unaryLoggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	start := time.Now()
	resp, err = handler(ctx, req)
	duration := time.Since(start)
	code := codes.OK
	if err != nil {
		if st, ok := status.FromError(err); ok {
			code = st.Code()
		}
	}
	log.Printf("gRPC call: method=%s duration=%v code=%v", info.FullMethod, duration, code)
	return resp, err
}

func streamLoggingInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	err := handler(srv, ss)
	duration := time.Since(start)
	code := codes.OK
	if err != nil {
		if st, ok := status.FromError(err); ok {
			code = st.Code()
		}
	}
	log.Printf("gRPC stream call: method=%s duration=%v code=%v", info.FullMethod, duration, code)
	return err
}

func (s *Server) Start() error {
	log.Printf("Starting gRPC server on %s", s.address)
	return s.server.Serve(s.listener)
}

func (s *Server) Stop() {
	log.Println("Stopping gRPC server")
	s.server.GracefulStop()
}

// GetGRPCServer 获取底层 gRPC 服务器（用于注册服务）
func (s *Server) GetServer() *grpc.Server {
	return s.server
}
