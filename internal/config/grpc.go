package config

import (
	"context"
	"fmt"
	"net"

	"flowguard/internal/limiter"
	pb "flowguard/internal/proto"
	"flowguard/internal/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// GRPCServer implements the FlowGuard gRPC service
type GRPCServer struct {
	pb.UnimplementedFlowGuardServiceServer
	rateLimiter *limiter.Manager
	server      *grpc.Server
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(rateLimiter *limiter.Manager) *GRPCServer {
	s := &GRPCServer{
		rateLimiter: rateLimiter,
		server:      grpc.NewServer(),
	}

	pb.RegisterFlowGuardServiceServer(s.server, s)
	
	// Enable reflection for debugging with tools like grpcurl
	reflection.Register(s.server)

	return s
}

// Start starts the gRPC server on the specified address
func (s *GRPCServer) Start(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	return s.server.Serve(listener)
}

// Stop gracefully stops the gRPC server
func (s *GRPCServer) Stop() {
	s.server.GracefulStop()
}

// SetClientConfig creates or updates a client's rate limiting configuration
func (s *GRPCServer) SetClientConfig(ctx context.Context, req *pb.SetClientConfigRequest) (*pb.SetClientConfigResponse, error) {
	if req.Config == nil {
		return &pb.SetClientConfigResponse{
			Success: false,
			Message: "Configuration is required",
		}, nil
	}

	if req.Config.ClientId == "" {
		return &pb.SetClientConfigResponse{
			Success: false,
			Message: "Client ID is required",
		}, nil
	}

	config := protoToClientConfig(req.Config)
	s.rateLimiter.SetClientConfig(config)

	return &pb.SetClientConfigResponse{
		Success: true,
		Message: "Client configuration updated successfully",
	}, nil
}

// GetClientConfig retrieves a client's configuration
func (s *GRPCServer) GetClientConfig(ctx context.Context, req *pb.GetClientConfigRequest) (*pb.GetClientConfigResponse, error) {
	if req.ClientId == "" {
		return &pb.GetClientConfigResponse{
			Found: false,
		}, nil
	}

	config, exists := s.rateLimiter.GetClientConfig(req.ClientId)
	if !exists {
		return &pb.GetClientConfigResponse{
			Found: false,
		}, nil
	}

	return &pb.GetClientConfigResponse{
		Config: clientConfigToProto(config),
		Found:  true,
	}, nil
}

// GetClientStats retrieves a client's usage statistics
func (s *GRPCServer) GetClientStats(ctx context.Context, req *pb.GetClientStatsRequest) (*pb.GetClientStatsResponse, error) {
	if req.ClientId == "" {
		return &pb.GetClientStatsResponse{
			Found: false,
		}, nil
	}

	stats, exists := s.rateLimiter.GetClientStats(req.ClientId)
	if !exists {
		return &pb.GetClientStatsResponse{
			Found: false,
		}, nil
	}

	return &pb.GetClientStatsResponse{
		Stats: clientStatsToProto(stats),
		Found: true,
	}, nil
}

// ListClients lists all configured clients
func (s *GRPCServer) ListClients(ctx context.Context, req *pb.ListClientsRequest) (*pb.ListClientsResponse, error) {
	configs := s.rateLimiter.GetAllClients()
	stats := s.rateLimiter.GetAllStats()

	var protoConfigs []*pb.ClientConfig
	var protoStats []*pb.ClientStats

	for _, config := range configs {
		protoConfigs = append(protoConfigs, clientConfigToProto(config))
	}

	for _, stat := range stats {
		protoStats = append(protoStats, clientStatsToProto(stat))
	}

	return &pb.ListClientsResponse{
		Clients: protoConfigs,
		Stats:   protoStats,
	}, nil
}

// DeleteClient removes a client configuration
func (s *GRPCServer) DeleteClient(ctx context.Context, req *pb.DeleteClientRequest) (*pb.DeleteClientResponse, error) {
	if req.ClientId == "" {
		return &pb.DeleteClientResponse{
			Success: false,
			Message: "Client ID is required",
		}, nil
	}

	deleted := s.rateLimiter.DeleteClient(req.ClientId)
	if !deleted {
		return &pb.DeleteClientResponse{
			Success: false,
			Message: "Client not found",
		}, nil
	}

	return &pb.DeleteClientResponse{
		Success: true,
		Message: "Client configuration deleted successfully",
	}, nil
}

// Helper functions to convert between proto and internal types

func protoToClientConfig(proto *pb.ClientConfig) *types.ClientConfig {
	config := &types.ClientConfig{
		ClientID: proto.ClientId,
		Enabled:  proto.Enabled,
	}

	if proto.Rpm != nil {
		rpm := *proto.Rpm
		config.RPM = &rpm
	}

	if proto.Tpm != nil {
		tpm := *proto.Tpm
		config.TPM = &tpm
	}

	return config
}

func clientConfigToProto(config *types.ClientConfig) *pb.ClientConfig {
	proto := &pb.ClientConfig{
		ClientId: config.ClientID,
		Enabled:  config.Enabled,
	}

	if config.RPM != nil {
		rpm := *config.RPM
		proto.Rpm = &rpm
	}

	if config.TPM != nil {
		tpm := *config.TPM
		proto.Tpm = &tpm
	}

	return proto
}

func clientStatsToProto(stats *types.ClientStats) *pb.ClientStats {
	return &pb.ClientStats{
		ClientId:         stats.ClientID,
		TotalRequests:    stats.TotalRequests,
		SuccessRequests:  stats.SuccessRequests,
		DroppedRequests:  stats.DroppedRequests,
		RpmDropped:       stats.RPMDropped,
		TpmDropped:       stats.TPMDropped,
		TokensUsed:       stats.TokensUsed,
		RpmRemaining:     stats.RPMRemaining,
		TpmRemaining:     stats.TPMRemaining,
		LastRequestTime:  stats.LastRequestTime.Unix(),
		AvgLatencyMs:     stats.AvgLatencyMs,
	}
} 