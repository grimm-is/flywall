// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package cloud_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pb "grimm.is/flywall/api/cloud/proto"
	agent "grimm.is/flywall/internal/cloud"
)

// mockDeviceServer implements a minimal DeviceService for testing
type mockDeviceServer struct {
	pb.UnimplementedDeviceServiceServer
}

func (s *mockDeviceServer) Enroll(ctx context.Context, req *pb.EnrollRequest) (*pb.EnrollResponse, error) {
	return &pb.EnrollResponse{
		OrgId:   "org_dummy_123",
		OrgName: "Acme Corp",
	}, nil
}

func (s *mockDeviceServer) Connect(stream pb.DeviceService_ConnectServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func TestAgentCloudIntegration(t *testing.T) {
	// 1. Start Cloud Server (In-process, using local mock)
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	s := grpc.NewServer()
	pb.RegisterDeviceServiceServer(s, &mockDeviceServer{})

	go func() {
		_ = s.Serve(lis)
	}()
	defer s.Stop()

	hubAddr := lis.Addr().String()

	// 2. Initialize Agent Client
	// Use insecure mode for test since we don't have certs yet
	c := agent.NewClient(hubAddr, "test-device-uuid", "", "")
	c.SetInsecure(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 3. Connect
	err = c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	// 4. Enroll
	resp, err := c.Enroll(ctx, "123456")
	require.NoError(t, err)
	assert.Equal(t, "org_dummy_123", resp.OrgId)

	// 5. Start Tunnel (Run in goroutine as it blocks)
	errChan := make(chan error, 1)
	go func() {
		errChan <- c.StartTunnel(ctx)
	}()

	// Wait a bit to ensure potential connection errors are caught
	select {
	case err := <-errChan:
		// If it returns early, it must be an error (or context timeout)
		// Note: ctx.Done() will cause Recv() to error, which is expected on test end
		if err != nil && ctx.Err() == nil {
			t.Errorf("Tunnel exited unexpectedly: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Success: tunnel is holding open
	}
}
