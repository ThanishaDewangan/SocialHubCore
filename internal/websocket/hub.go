package websocket

import (
        "encoding/json"
        "sync"

        "github.com/google/uuid"
)

type Event struct {
        Type    string      `json:"type"`
        Payload interface{} `json:"payload"`
}

type ViewEvent struct {
        StoryID  uuid.UUID `json:"story_id"`
        ViewerID uuid.UUID `json:"viewer_id"`
        ViewedAt string    `json:"viewed_at"`
}

type ReactionEvent struct {
        StoryID uuid.UUID `json:"story_id"`
        UserID  uuid.UUID `json:"user_id"`
        Emoji   string    `json:"emoji"`
}

type Hub struct {
        clients    map[uuid.UUID]map[*Client]bool
        broadcast  chan *Message
        register   chan *Client
        unregister chan *Client
        mu         sync.RWMutex
}

type Message struct {
        UserID  uuid.UUID
        Payload []byte
}

func NewHub() *Hub {
        return &Hub{
                clients:    make(map[uuid.UUID]map[*Client]bool),
                broadcast:  make(chan *Message, 256),
                register:   make(chan *Client),
                unregister: make(chan *Client),
        }
}

func (h *Hub) Run() {
        for {
                select {
                case client := <-h.register:
                        h.mu.Lock()
                        if _, ok := h.clients[client.UserID]; !ok {
                                h.clients[client.UserID] = make(map[*Client]bool)
                        }
                        h.clients[client.UserID][client] = true
                        h.mu.Unlock()

                case client := <-h.unregister:
                        h.mu.Lock()
                        if clients, ok := h.clients[client.UserID]; ok {
                                if _, ok := clients[client]; ok {
                                        delete(clients, client)
                                        close(client.send)
                                        if len(clients) == 0 {
                                                delete(h.clients, client.UserID)
                                        }
                                }
                        }
                        h.mu.Unlock()

                case message := <-h.broadcast:
                        h.mu.RLock()
                        if clients, ok := h.clients[message.UserID]; ok {
                                for client := range clients {
                                        select {
                                        case client.send <- message.Payload:
                                        default:
                                                close(client.send)
                                                delete(clients, client)
                                        }
                                }
                        }
                        h.mu.RUnlock()
                }
        }
}

func (h *Hub) RegisterClient(client *Client) {
        h.register <- client
}

func (h *Hub) SendToUser(userID uuid.UUID, event Event) {
        payload, err := json.Marshal(event)
        if err != nil {
                return
        }

        h.broadcast <- &Message{
                UserID:  userID,
                Payload: payload,
        }
}
