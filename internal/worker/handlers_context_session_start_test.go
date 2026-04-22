package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/thebtf/engram/internal/grpcserver"
	pb "github.com/thebtf/engram/proto/engram/v1"
)

type stubSessionStartContextServer struct {
	grpcserver.Server
	resp *pb.GetSessionStartContextResponse
	err  error
	req  *pb.GetSessionStartContextRequest
}

func (s *stubSessionStartContextServer) GetSessionStartContext(_ context.Context, req *pb.GetSessionStartContextRequest) (*pb.GetSessionStartContextResponse, error) {
	s.req = req
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func TestHandleSessionStartContextStatic_HappyPath(t *testing.T) {
	t.Parallel()

	generatedAt := "2026-04-22T13:00:00Z"
	server := &stubSessionStartContextServer{
		resp: &pb.GetSessionStartContextResponse{
			Issues: []*pb.SessionStartIssue{{
				Id:            1,
				Title:         "Issue title",
				Status:        "open",
				Priority:      "high",
				Type:          "bug",
				SourceProject: "source",
				TargetProject: "engram",
			}},
			Rules: []*pb.SessionStartRule{{
				Id:      2,
				Content: "Rule content",
				Project: "engram",
			}},
			Memories: []*pb.SessionStartMemory{{
				Id:      3,
				Project: "engram",
				Content: "Memory content",
			}},
			GeneratedAt: mustProtoTimestamp(t, generatedAt),
		},
	}
	service := &Service{grpcInternalServer: server}

	req := httptest.NewRequest(http.MethodGet, "/api/context/session-start?project=engram", nil)
	w := httptest.NewRecorder()

	service.handleSessionStartContextStatic(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, server.req)
	assert.Equal(t, "engram", server.req.GetProject())

	var body sessionStartCompatibilityResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Issues, 1)
	require.Len(t, body.Rules, 1)
	require.Len(t, body.Memories, 1)
	assert.Equal(t, generatedAt, body.GeneratedAt)
	assert.Equal(t, "Issue title", body.Issues[0]["title"])
	assert.Equal(t, "Rule content", body.Rules[0]["content"])
	assert.Equal(t, "Memory content", body.Memories[0]["content"])
}

func TestHandleSessionStartContextStatic_MapsGrpcErrors(t *testing.T) {
	t.Parallel()

	service := &Service{grpcInternalServer: &stubSessionStartContextServer{
		err: grpcstatus.Error(codes.Unavailable, "database not ready"),
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/context/session-start?project=engram", nil)
	w := httptest.NewRecorder()

	service.handleSessionStartContextStatic(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "database not ready")
}

func mustProtoTimestamp(t *testing.T, iso string) *timestamppb.Timestamp {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, iso)
	require.NoError(t, err)
	return timestamppb.New(parsed)
}
