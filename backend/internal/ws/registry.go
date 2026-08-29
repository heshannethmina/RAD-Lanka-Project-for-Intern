package ws

import (
	"log"
	"regexp"
)

// roomIDPattern keeps room IDs to something safe to log and to put in a URL.
// It also bounds how much garbage a caller can turn into live goroutines.
var roomIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidRoomID reports whether id is acceptable as a room name.
func ValidRoomID(id string) bool { return roomIDPattern.MatchString(id) }

// roomEntry is a live room: its hub, and how many clients the registry has
// handed that hub out to. The count is the registry's own bookkeeping and is
// deliberately separate from the hub's client set — each is owned by exactly
// one goroutine.
type roomEntry struct {
	hub     *Hub
	clients int
}

type joinRequest struct {
	roomID string
	reply  chan *Hub
}

// Registry owns the set of live rooms.
//
// Both joining and leaving pass through the Run goroutine, which is what
// makes teardown safe: the decision to close a room and the decision to hand
// that room to a new client are made by the same goroutine, in order, so a
// client can never be given a hub that is already shutting down.
type Registry struct {
	// Owned by Run.
	rooms map[string]*roomEntry

	join  chan joinRequest
	leave chan string
	probe chan chan int
}

func NewRegistry() *Registry {
	return &Registry{
		rooms: make(map[string]*roomEntry),
		join:  make(chan joinRequest),
		leave: make(chan string),
		probe: make(chan chan int),
	}
}

// Join returns the hub for roomID, starting it if this is the first client.
// Every Join must be paired with exactly one Leave.
func (r *Registry) Join(roomID string) *Hub {
	reply := make(chan *Hub)
	r.join <- joinRequest{roomID: roomID, reply: reply}
	return <-reply
}

// Leave releases one client's claim on a room. The last one out closes it.
func (r *Registry) Leave(roomID string) { r.leave <- roomID }

// Run owns the room map. It never returns.
func (r *Registry) Run() {
	for {
		select {
		case req := <-r.join:
			e, ok := r.rooms[req.roomID]
			if !ok {
				e = &roomEntry{hub: NewHub(req.roomID)}
				r.rooms[req.roomID] = e
				go e.hub.Run()
				log.Printf("ws: room %q opened", req.roomID)
			}
			e.clients++
			req.reply <- e.hub

		case roomID := <-r.leave:
			e, ok := r.rooms[roomID]
			if !ok {
				// Not possible today; guard rather than panic on a miscount.
				log.Printf("ws: leave for unknown room %q", roomID)
				continue
			}
			e.clients--
			if e.clients > 0 {
				continue
			}
			// Last client out. Removing from the map before stopping the hub
			// means no later Join can find it, and because a client only
			// leaves after its unregister has been consumed, there is nothing
			// still in flight to the hub.
			delete(r.rooms, roomID)
			e.hub.Stop()
			log.Printf("ws: room %q closed", roomID)

		case reply := <-r.probe:
			reply <- len(r.rooms)
		}
	}
}

// Rooms reports the number of live rooms. For tests and diagnostics; the
// count is read inside the Run goroutine like every other access to the map.
func (r *Registry) Rooms() int {
	reply := make(chan int)
	r.probe <- reply
	return <-reply
}
