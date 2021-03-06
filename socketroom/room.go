package socketroom

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// Room will be the place clients use to create a pointing session.
type Room struct {
	// The Hub handles all rooms.
	hub *Hub

	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan GameMessage

	// reads raw wrapped GameMessage with decoded payload, for determineGameAction function
	determineGameAction chan GameEvent

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	Name string

	// scale in which points are voted in, during the session.
	PointScale string
}

// GameEvent Struct to contain rawPayload for reducer action
type GameEvent struct {
	gameMessage GameMessage
	rawPayload  json.RawMessage
}

// CreateRoom creates a new room and registers it with the hub.
func CreateRoom(hub *Hub, pS string) *Room {
	fmt.Println("pS", pS)
	room := &Room{
		hub:                 hub,
		clients:             make(map[*Client]bool),
		register:            make(chan *Client),
		unregister:          make(chan *Client),
		Name:                createRoomName(),
		broadcast:           make(chan GameMessage),
		determineGameAction: make(chan GameEvent),
		PointScale:          pS,
	}
	room.hub.register <- room
	return room
}

func (r *Room) sendPlayers() []PlayerStatus {
	ps := []PlayerStatus{}
	for p := range r.clients {
		if !p.observer {
			value := PlayerStatus{p.Name, p.CurrentPoint, p.ID}
			ps = append(ps, value)
		}
	}
	return ps
}

func (r *Room) sendObservers() []Observer {
	observers := []Observer{}
	for c := range r.clients {
		if c.observer {
			o := Observer{c.Name, c.ID}
			observers = append(observers, o)
		}
	}
	return observers
}

// Start begins the goroutine and channels for the room
func (r *Room) Start() {
	for {
		select {
		case client := <-r.register:
			r.clients[client] = true
			fmt.Println("registered with room", client.Name)
			joinMsg := GameMessage{
				Event: joinRoom,
				Payload: PlayerUpdate{
					Players:    r.sendPlayers(),
					Observers:  r.sendObservers(),
					PointScale: r.PointScale,
				},
			}
			for client := range r.clients {
				client.send <- joinMsg
			}

		case client := <-r.unregister:
			delete(r.clients, client)
			close(client.send)
			exitMsg := GameMessage{
				Event: leaveRoom,
				Payload: PlayerUpdate{
					Players:   r.sendPlayers(),
					Observers: r.sendObservers(),
				},
			}
			for client := range r.clients {
				client.send <- exitMsg
			}
		case gameEvent := <-r.determineGameAction:
			determineGameAction(r, gameEvent.gameMessage, gameEvent.rawPayload)
		case gameMessage := <-r.broadcast:
			for client := range r.clients {
				select {
				case client.send <- gameMessage:
				default:
					close(client.send)
					delete(r.clients, client)
				}
			}
		}
	}
}

func (r *Room) updateVote(point string, id string) {
	clientsVoted := 0
	for c := range r.clients {
		if c.ID == id && !c.observer {
			c.CurrentPoint = point
		}
	}
	for c := range r.clients {
		if !c.observer {
			if c.CurrentPoint == "" {
				break
			}
			clientsVoted++
		}
	}
	if clientsVoted == r.countPlayers() {
		fmt.Println("All clients voted")
		msg := GameMessage{Event: revealPoints}
		for c := range r.clients {
			c.send <- msg
		}
	}

}

func (r *Room) countPlayers() int {
	pCount := 0
	for c := range r.clients {
		if !c.observer {
			pCount++
		}
	}
	return pCount
}

// ListClients prints to console the clients in the room
func (r *Room) ListClients() {
	for k := range r.clients {
		fmt.Println("Clients", k)
	}
}

func (r *Room) clearPoints() {
	for c := range r.clients {
		c.CurrentPoint = ""
	}
	fmt.Println("All points cleared")
}

// Logic to create random room name
const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func createRoomName() string {
	return stringWithCharset(6, charset)
}
