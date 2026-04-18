package broadcaster

import (
	"context"
	"flowforge/pkg/redis"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type SSEBroadcaster struct {
	// tenantClients maps tenantID -> map[clientChan]bool
	tenantClients map[string]map[chan []byte]bool
	// subscriptions maps tenantID -> cancel function for Redis subscription
	subscriptions map[string]context.CancelFunc
	mutex         sync.RWMutex
	cache         redis.Cache
	log           *zap.Logger
}

var (
	instance *SSEBroadcaster
	once     sync.Once
)

func Init(cache redis.Cache, log *zap.Logger) *SSEBroadcaster {
	once.Do(func() {
		instance = &SSEBroadcaster{
			tenantClients: make(map[string]map[chan []byte]bool),
			subscriptions: make(map[string]context.CancelFunc),
			cache:         cache,
			log:           log,
		}
	})
	return instance
}

func Get() *SSEBroadcaster {
	if instance == nil {
		panic("broadcaster not initialized. call Init first")
	}
	return instance
}

func (b *SSEBroadcaster) Register(tenantID string, clientChan chan []byte) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if _, ok := b.tenantClients[tenantID]; !ok {
		b.tenantClients[tenantID] = make(map[chan []byte]bool)
		// Start Redis subscription for this tenant if not already started
		ctx, cancel := context.WithCancel(context.Background())
		b.subscriptions[tenantID] = cancel
		go b.listenRedis(ctx, tenantID)
	}
	b.tenantClients[tenantID][clientChan] = true
}

func (b *SSEBroadcaster) Unregister(tenantID string, clientChan chan []byte) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if consumers, ok := b.tenantClients[tenantID]; ok {
		delete(consumers, clientChan)
		close(clientChan)

		if len(consumers) == 0 {
			delete(b.tenantClients, tenantID)
			// Stop Redis subscription if no more clients for this tenant
			if cancel, ok := b.subscriptions[tenantID]; ok {
				cancel()
				delete(b.subscriptions, tenantID)
			}
		}
	}
}

func (b *SSEBroadcaster) listenRedis(ctx context.Context, tenantID string) {
	channel := fmt.Sprintf("flowforge:events:tenant_%s", tenantID)
	pubsub := b.cache.Subscribe(ctx, channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			b.broadcastLocal(tenantID, []byte(msg.Payload))
		}
	}
}

func (b *SSEBroadcaster) broadcastLocal(tenantID string, data []byte) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	if consumers, ok := b.tenantClients[tenantID]; ok {
		for clientChan := range consumers {
			select {
			case clientChan <- data:
			default:
				// If channel is blocked, ignore to prevent blocking broadcaster
			}
		}
	}
}

// BroadcastToRedis publishes an event to Redis, which will be picked up by all API instances
func (b *SSEBroadcaster) BroadcastToRedis(ctx context.Context, tenantID string, event any) error {
	channel := fmt.Sprintf("flowforge:events:tenant_%s", tenantID)
	return b.cache.Publish(ctx, channel, event)
}

// Broadcast is kept for backward compatibility or global events (if needed), currently not used
func (b *SSEBroadcaster) Broadcast(event any) {
	// Legacy global broadcast - could be implemented if needed
}
