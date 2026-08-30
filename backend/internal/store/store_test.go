package store_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/heshannethmina/interview-platform/backend/internal/auth"
	"github.com/heshannethmina/interview-platform/backend/internal/store"
)

// These are integration tests: they need a real Postgres, because the things
// most worth testing here are the constraints and the SQL, and a fake would
// exercise neither. They skip when TEST_DATABASE_URL is unset so that
// `go test ./...` still passes on a machine without a database.
//
//	docker compose -f docker-compose.app.yml up -d --wait
//	TEST_DATABASE_URL='postgres://syncr:syncrdev@localhost:5433/syncr' go test ./internal/store/
func open(t *testing.T) *store.Store {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping store integration tests")
	}

	ctx := context.Background()
	s, err := store.Open(ctx, url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(s.Close)

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

// uniqueEmail keeps tests independent without truncating tables, so a failing
// run leaves its rows behind to be inspected.
func uniqueEmail(t *testing.T) string {
	t.Helper()
	tok, _, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return tok[:16] + "@example.test"
}

func newUser(t *testing.T, s *store.Store) *store.User {
	t.Helper()
	hash, err := auth.HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := s.CreateUser(context.Background(), uniqueEmail(t), hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := open(t)
	// open already migrated once; a second run must be a no-op rather than an
	// error, because the server runs migrations on every boot.
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestCreateUserRejectsDuplicateEmail(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)

	_, err := s.CreateUser(ctx, u.Email, u.PasswordHash)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate email: got %v, want ErrConflict", err)
	}

	// Case-insensitively too, or two accounts could differ only by casing and
	// login would not know which was meant.
	_, err = s.CreateUser(ctx, strings.ToUpper(u.Email), u.PasswordHash)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("case-variant email: got %v, want ErrConflict", err)
	}
}

func TestUserByEmailIsCaseInsensitive(t *testing.T) {
	s := open(t)
	u := newUser(t, s)

	got, err := s.UserByEmail(context.Background(), strings.ToUpper(u.Email))
	if err != nil {
		t.Fatalf("by email: %v", err)
	}
	if got.ID != u.ID {
		t.Fatalf("got user %d, want %d", got.ID, u.ID)
	}
}

func TestUnknownUserIsNotFound(t *testing.T) {
	s := open(t)
	_, err := s.UserByEmail(context.Background(), "nobody@example.test")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)

	tok, hash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if err := s.CreateSession(ctx, u.ID, hash, time.Now().Add(store.SessionTTL)); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := s.UserBySessionToken(ctx, auth.HashToken(tok))
	if err != nil {
		t.Fatalf("resolve session: %v", err)
	}
	if got.ID != u.ID {
		t.Fatalf("session resolved to user %d, want %d", got.ID, u.ID)
	}

	if err := s.DeleteSession(ctx, hash); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := s.UserBySessionToken(ctx, hash); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("after logout: got %v, want ErrNotFound", err)
	}
	// Logging out twice is not an error.
	if err := s.DeleteSession(ctx, hash); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestExpiredSessionDoesNotAuthenticate(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)

	_, hash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	// Already expired when written: expiry is enforced by the query, so this
	// must not authenticate even though the row exists.
	if err := s.CreateSession(ctx, u.ID, hash, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if _, err := s.UserBySessionToken(ctx, hash); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired session: got %v, want ErrNotFound", err)
	}

	n, err := s.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n < 1 {
		t.Fatalf("sweep removed %d rows, want at least 1", n)
	}
}

func newRoom(t *testing.T, s *store.Store, ownerID int64) (*store.Room, string) {
	t.Helper()
	id, err := auth.NewRoomID()
	if err != nil {
		t.Fatalf("room id: %v", err)
	}
	invite, inviteHash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	r, err := s.CreateRoom(context.Background(), store.NewRoom{
		ID:              id,
		OwnerID:         ownerID,
		Title:           "Backend screen",
		Language:        "python",
		Duration:        30 * time.Minute,
		InviteTokenHash: inviteHash,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	return r, invite
}

func TestGeneratedRoomIDSatisfiesTheConstraint(t *testing.T) {
	s := open(t)
	u := newUser(t, s)
	// The CHECK in the migration mirrors ws.ValidRoomID. If NewRoomID ever
	// produced something outside that alphabet, this insert would fail.
	for range 20 {
		newRoom(t, s, u.ID)
	}
}

func TestRoomByInviteRequiresBothHalves(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)
	r, invite := newRoom(t, s, u.ID)

	got, err := s.RoomByInvite(ctx, r.ID, auth.HashToken(invite))
	if err != nil {
		t.Fatalf("valid invite: %v", err)
	}
	if got.ID != r.ID {
		t.Fatalf("got room %q, want %q", got.ID, r.ID)
	}

	// A valid token for a different room must not open this one.
	other, otherInvite := newRoom(t, s, u.ID)
	if _, err := s.RoomByInvite(ctx, r.ID, auth.HashToken(otherInvite)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("token from room %q opened room %q: %v", other.ID, r.ID, err)
	}

	if _, err := s.RoomByInvite(ctx, r.ID, auth.HashToken("not-a-token")); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("bad token: got %v, want ErrNotFound", err)
	}
}

func TestClosedRoomRefusesInvite(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)
	r, invite := newRoom(t, s, u.ID)

	if err := s.CloseRoom(ctx, r.ID, u.ID); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := s.RoomByInvite(ctx, r.ID, auth.HashToken(invite)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("closed room accepted invite: %v", err)
	}
	// Closing twice reports not-found rather than silently succeeding.
	if err := s.CloseRoom(ctx, r.ID, u.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second close: got %v, want ErrNotFound", err)
	}
}

func TestCloseRoomIsScopedToOwner(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	owner := newUser(t, s)
	stranger := newUser(t, s)
	r, _ := newRoom(t, s, owner.ID)

	if err := s.CloseRoom(ctx, r.ID, stranger.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stranger closed another's room: %v", err)
	}
	fresh, err := s.RoomByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("room by id: %v", err)
	}
	if !fresh.Open() {
		t.Fatal("room was closed by a non-owner")
	}
}

func TestRotateInviteRevokesTheOldLink(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)
	r, oldInvite := newRoom(t, s, u.ID)

	newInvite, newHash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if err := s.RotateInvite(ctx, r.ID, u.ID, newHash); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	if _, err := s.RoomByInvite(ctx, r.ID, auth.HashToken(oldInvite)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("old invite still works: %v", err)
	}
	if _, err := s.RoomByInvite(ctx, r.ID, auth.HashToken(newInvite)); err != nil {
		t.Fatalf("new invite rejected: %v", err)
	}
}

func TestRoomsByOwnerIsScopedAndOrdered(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	mine := newUser(t, s)
	theirs := newUser(t, s)

	var created []string
	for range 3 {
		r, _ := newRoom(t, s, mine.ID)
		created = append(created, r.ID)
	}
	newRoom(t, s, theirs.ID)

	rooms, err := s.RoomsByOwner(ctx, mine.ID, 50)
	if err != nil {
		t.Fatalf("rooms by owner: %v", err)
	}
	if len(rooms) != len(created) {
		t.Fatalf("got %d rooms, want %d", len(rooms), len(created))
	}
	for _, r := range rooms {
		if r.OwnerID != mine.ID {
			t.Fatalf("listing leaked room %q owned by %d", r.ID, r.OwnerID)
		}
	}
	for i := 1; i < len(rooms); i++ {
		if rooms[i-1].CreatedAt.Before(rooms[i].CreatedAt) {
			t.Fatal("rooms are not newest-first")
		}
	}

	// An interviewer with no rooms gets an empty slice, not nil, so the JSON
	// is [] rather than null.
	empty, err := s.RoomsByOwner(ctx, theirs.ID+1_000_000, 50)
	if err != nil {
		t.Fatalf("rooms by owner: %v", err)
	}
	if empty == nil {
		t.Fatal("want empty slice, got nil")
	}
}

func TestNewRoomHasNotStartedUntilSomebodyJoins(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)
	r, _ := newRoom(t, s, u.ID)

	if r.StartedAt != nil {
		t.Fatal("a freshly created room reported itself started")
	}
	if !r.EndsAt().IsZero() {
		t.Fatal("a room nobody has opened already has a deadline")
	}

	started, err := s.StartRoom(ctx, r.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if started.StartedAt == nil {
		t.Fatal("StartRoom did not stamp the start")
	}
	if started.EndsAt().IsZero() {
		t.Fatal("a started room has no deadline")
	}
}

// The clock must not restart when the second person arrives, or an interview
// could be extended indefinitely by rejoining.
func TestStartRoomIsIdempotent(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)
	r, _ := newRoom(t, s, u.ID)

	first, err := s.StartRoom(ctx, r.ID)
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	second, err := s.StartRoom(ctx, r.ID)
	if err != nil {
		t.Fatalf("second start: %v", err)
	}
	if !first.StartedAt.Equal(*second.StartedAt) {
		t.Fatalf("the clock restarted: %v then %v", first.StartedAt, second.StartedAt)
	}
}

// Counting is what the plan allowance is enforced from, so it must be scoped
// to the owner.
func TestCountRoomsIsPerOwner(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	mine := newUser(t, s)
	theirs := newUser(t, s)

	for range 3 {
		newRoom(t, s, mine.ID)
	}
	newRoom(t, s, theirs.ID)

	n, err := s.CountRooms(ctx, mine.ID, true)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Fatalf("counted %d, want 3", n)
	}
}

// A booking is not usage: a room nobody opened must contribute nothing.
func TestUsedDurationIgnoresRoomsThatNeverStarted(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	u := newUser(t, s)
	newRoom(t, s, u.ID)

	used, err := s.UsedDuration(ctx, u.ID, true)
	if err != nil {
		t.Fatalf("used: %v", err)
	}
	if used != 0 {
		t.Fatalf("an unopened room counted %v of usage", used)
	}
}

func TestNewUserStartsOnTheFreePlan(t *testing.T) {
	s := open(t)
	u := newUser(t, s)
	if u.Plan != "free" {
		t.Fatalf("plan = %q, want free", u.Plan)
	}
}
