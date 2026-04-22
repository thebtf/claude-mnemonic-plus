package grpcserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "github.com/thebtf/engram/proto/engram/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type staticMCPHandler struct {
	serverName    string
	serverVersion string
}

func (h staticMCPHandler) HandleToolCall(_ context.Context, _ string, _ []byte) ([]byte, bool, error) {
	return nil, false, nil
}

func (h staticMCPHandler) ToolDefinitions() []ToolDef {
	return nil
}

func (h staticMCPHandler) ServerInfo() (string, string) {
	return h.serverName, h.serverVersion
}

func TestNegotiateVersion_CompatibleMajor(t *testing.T) {
	t.Parallel()

	srv := &Server{handler: staticMCPHandler{serverName: "engram", serverVersion: "v5.0.0"}}
	resp, err := srv.NegotiateVersion(context.Background(), &pb.NegotiateVersionRequest{ClientVersion: "5.1.2"})
	require.NoError(t, err)
	assert.True(t, resp.Compatible)
	assert.Equal(t, "v5.0.0", resp.ServerVersion)
	assert.Empty(t, resp.IncompatReason)
}

func TestNegotiateVersion_IncompatibleMajor(t *testing.T) {
	t.Parallel()

	srv := &Server{handler: staticMCPHandler{serverName: "engram", serverVersion: "v5.0.0"}}
	resp, err := srv.NegotiateVersion(context.Background(), &pb.NegotiateVersionRequest{ClientVersion: "v4.9.9"})
	require.NoError(t, err)
	assert.False(t, resp.Compatible)
	assert.Equal(t, "v5.0.0", resp.ServerVersion)
	assert.Contains(t, resp.IncompatReason, "client major version 4")
	assert.Contains(t, resp.IncompatReason, "server major version 5")
}

func TestNegotiateVersion_InvalidArgument(t *testing.T) {
	t.Parallel()

	srv := &Server{handler: staticMCPHandler{serverName: "engram", serverVersion: "v5.0.0"}}

	_, err := srv.NegotiateVersion(context.Background(), &pb.NegotiateVersionRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = srv.NegotiateVersion(context.Background(), &pb.NegotiateVersionRequest{ClientVersion: "banana"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestParseMajorVersion(t *testing.T) {
	t.Parallel()

	major, err := parseMajorVersion("v12.3.4")
	require.NoError(t, err)
	assert.Equal(t, 12, major)

	major, err = parseMajorVersion("7")
	require.NoError(t, err)
	assert.Equal(t, 7, major)

	_, err = parseMajorVersion("")
	require.Error(t, err)

	_, err = parseMajorVersion("v")
	require.Error(t, err)

	_, err = parseMajorVersion("x.1.2")
	require.Error(t, err)
}
