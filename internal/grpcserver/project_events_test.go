package grpcserver

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/thebtf/engram/internal/worker/projectevents"
	pb "github.com/thebtf/engram/proto/engram/v1"
)

const bufSize = 1 << 20 // 1 MiB

// newBufconnServer starts an in-process gRPC server using bufconn and returns:
//   - The underlying *Server (for direct bus access in tests)
//   - A gRPC client connected over the bufconn
//   - A cancel function that stops the server and closes the connection
func newBufconnServer(t *testing.T, bus *projectevents.Bus) (*Server, pb.EngramServiceClient, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)

	// Build the server using the package-internal constructor so we can inject a bus.
	internalSrv := &Server{bus: bus}
	// handler/token not needed for ProjectEvents tests.

	gs := grpc.NewServer()
	pb.RegisterEngramServiceServer(gs, internalSrv)

	go func() {
		_ = gs.Serve(lis)
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}

	client := pb.NewEngramServiceClient(conn)

	stop := func() {
		conn.Close()
		gs.Stop()
		lis.Close()
	}
	return internalSrv, client, stop
}

func TestProjectEvents_SinceEventIdRejected(t *testing.T) {
	t.Parallel()

	bus := &projectevents.Bus{}
	_, client, stop := newBufconnServer(t, bus)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.ProjectEvents(ctx, &pb.ProjectEventsRequest{
		ClientId:     "daemon-test",
		SinceEventId: "some-id",
	})
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	_, err = stream.Recv()
	if err == nil {
		t.Fatal("expected OUT_OF_RANGE error for non-empty since_event_id")
	}
	if status.Code(err) != codes.OutOfRange {
		t.Errorf("expected codes.OutOfRange, got %v", status.Code(err))
	}
}

func TestProjectEvents_HappyPath(t *testing.T) {
	t.Parallel()

	bus := &projectevents.Bus{}
	_, client, stop := newBufconnServer(t, bus)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.ProjectEvents(ctx, &pb.ProjectEventsRequest{
		ClientId: "daemon-happy",
	})
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	// Emit one event via the bus.
	go func() {
		time.Sleep(50 * time.Millisecond) // give stream time to subscribe
		bus.Emit(projectevents.Event{
			EventType:       projectevents.EventTypeRemoved,
			ProjectID:       "proj-happy",
			TimestampUnixMs: time.Now().UnixMilli(),
		})
	}()

	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if ev.GetProjectId() != "proj-happy" {
		t.Errorf("expected project_id=proj-happy, got %s", ev.GetProjectId())
	}
	if ev.GetEventType() != pb.ProjectEventType_PROJECT_EVENT_TYPE_REMOVED {
		t.Errorf("expected PROJECT_EVENT_TYPE_REMOVED, got %v", ev.GetEventType())
	}
	if ev.GetEventId() == "" {
		t.Error("expected non-empty event_id")
	}
}

func TestProjectEvents_MultipleEvents(t *testing.T) {
	t.Parallel()

	bus := &projectevents.Bus{}
	_, client, stop := newBufconnServer(t, bus)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.ProjectEvents(ctx, &pb.ProjectEventsRequest{
		ClientId: "daemon-multi",
	})
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	const n = 3
	go func() {
		time.Sleep(50 * time.Millisecond)
		for i := 0; i < n; i++ {
			bus.Emit(projectevents.Event{
				EventType:       projectevents.EventTypeRemoved,
				ProjectID:       "proj-multi",
				TimestampUnixMs: time.Now().UnixMilli(),
			})
		}
	}()

	for i := 0; i < n; i++ {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv %d: %v", i, err)
		}
		if ev.GetProjectId() != "proj-multi" {
			t.Errorf("event %d: expected proj-multi, got %s", i, ev.GetProjectId())
		}
	}
}

func TestProjectEvents_ContextCancel(t *testing.T) {
	t.Parallel()

	bus := &projectevents.Bus{}
	_, client, stop := newBufconnServer(t, bus)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	stream, err := client.ProjectEvents(ctx, &pb.ProjectEventsRequest{
		ClientId: "daemon-cancel",
	})
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	// Cancel the context after a brief delay — stream should terminate cleanly.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err = stream.Recv()
	if err == nil {
		t.Fatal("expected error after context cancel")
	}
	// Context cancellation should give Canceled or DeadlineExceeded.
	code := status.Code(err)
	if code != codes.Canceled && code != codes.DeadlineExceeded && code != codes.Unavailable {
		t.Errorf("expected Canceled/DeadlineExceeded/Unavailable after context cancel, got %v", code)
	}
}
