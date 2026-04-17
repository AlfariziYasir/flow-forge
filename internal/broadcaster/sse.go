package broadcaster

import (
	"encoding/json"
	"sync"
)

type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type SSEBroadcaster struct {
	clients map[chan []byte]bool
	mutex   sync.RWMutex
}

var (
	instance *SSEBroadcaster
	once     sync.Once
)

func Get() *SSEBroadcaster {
	once.Do(func() {
		instance = &SSEBroadcaster{
			clients: make(map[chan []byte]bool),
		}
	})
	return instance
}

func (b *SSEBroadcaster) Register(clientChan chan []byte) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.clients[clientChan] = true
}

func (b *SSEBroadcaster) Unregister(clientChan chan []byte) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	delete(b.clients, clientChan)
	close(clientChan)
}

func (b *SSEBroadcaster) Broadcast(event any) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	for clientChan := range b.clients {
		select {
		case clientChan <- data:
		default:
			// If channel is blocked, ignore to prevent blocking broadcaster
		}
	}
}
