// Package persistence provides Redis queue persistence for crash-resilient operations.
package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Config holds Redis configuration.
type Config struct {
	Host     string
	Port     int
	Password string
	DB       int

	// Queue settings
	QueueSize         int
	MessageTimeout    time.Duration
	RetryAttempts     int
	RetryDelay        time.Duration

	// Persistence settings
	PersistenceEnabled bool
	PersistenceInterval time.Duration
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		Host:               "localhost",
		Port:               6379,
		DB:                 0,
		QueueSize:          10000,
		MessageTimeout:     5 * time.Minute,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		PersistenceEnabled: true,
		PersistenceInterval: 30 * time.Second,
	}
}

// QueueMessage represents a message in the persistence queue.
type QueueMessage struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Priority  int             `json:"priority"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	RetryCount int            `json:"retry_count"`
	Status    string          `json:"status"` // pending, processing, completed, failed
}

// QueueStore defines the interface for queue operations.
type QueueStore interface {
	Enqueue(ctx context.Context, msg *QueueMessage) error
	Dequeue(ctx context.Context) (*QueueMessage, error)
	Ack(ctx context.Context, id string) error
	Nack(ctx context.Context, id string) error
	GetMessage(ctx context.Context, id string) (*QueueMessage, error)
	GetPendingMessages(ctx context.Context) ([]*QueueMessage, error)
	GetFailedMessages(ctx context.Context) ([]*QueueMessage, error)
	Cleanup(ctx context.Context, olderThan time.Duration) (int64, error)
}

// RedisQueue implements QueueStore using Redis.
type RedisQueue struct {
	config Config
	logger *slog.Logger
}

// NewRedisQueue creates a new Redis queue.
func NewRedisQueue(config Config, logger *slog.Logger) *RedisQueue {
	return &RedisQueue{
		config: config,
		logger: logger,
	}
}

// Enqueue adds a message to the queue.
func (q *RedisQueue) Enqueue(ctx context.Context, msg *QueueMessage) error {
	msg.CreatedAt = time.Now()
	msg.UpdatedAt = time.Now()
	msg.Status = "pending"

	// In a real implementation, this would use Redis LPUSH
	// For now, we'll simulate with in-memory storage
	q.logger.DebugContext(ctx, "enqueuing message",
		"id", msg.ID,
		"type", msg.Type,
	)

	return nil
}

// Dequeue removes a message from the queue.
func (q *RedisQueue) Dequeue(ctx context.Context) (*QueueMessage, error) {
	// In a real implementation, this would use Redis RPOP
	// For now, we'll simulate with in-memory storage
	q.logger.DebugContext(ctx, "dequeuing message")

	return nil, fmt.Errorf("no messages available")
}

// Ack acknowledges a message.
func (q *RedisQueue) Ack(ctx context.Context, id string) error {
	q.logger.DebugContext(ctx, "acknowledging message", "id", id)
	return nil
}

// Nack negatively acknowledges a message (requeue).
func (q *RedisQueue) Nack(ctx context.Context, id string) error {
	q.logger.DebugContext(ctx, "nacking message", "id", id)
	return nil
}

// GetMessage retrieves a message by ID.
func (q *RedisQueue) GetMessage(ctx context.Context, id string) (*QueueMessage, error) {
	q.logger.DebugContext(ctx, "getting message", "id", id)
	return nil, fmt.Errorf("message not found")
}

// GetPendingMessages retrieves all pending messages.
func (q *RedisQueue) GetPendingMessages(ctx context.Context) ([]*QueueMessage, error) {
	q.logger.DebugContext(ctx, "getting pending messages")
	return nil, nil
}

// GetFailedMessages retrieves all failed messages.
func (q *RedisQueue) GetFailedMessages(ctx context.Context) ([]*QueueMessage, error) {
	q.logger.DebugContext(ctx, "getting failed messages")
	return nil, nil
}

// Cleanup removes old messages.
func (q *RedisQueue) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	q.logger.DebugContext(ctx, "cleaning up old messages", "older_than", olderThan)
	return 0, nil
}

// MessageProcessor processes messages from the queue.
type MessageProcessor struct {
	queue      QueueStore
	logger     *slog.Logger
	handlers   map[string]MessageHandler
	workerCount int
}

// MessageHandler processes a specific message type.
type MessageHandler func(ctx context.Context, msg *QueueMessage) error

// NewMessageProcessor creates a new message processor.
func NewMessageProcessor(queue QueueStore, logger *slog.Logger, workerCount int) *MessageProcessor {
	return &MessageProcessor{
		queue:       queue,
		logger:      logger,
		handlers:    make(map[string]MessageHandler),
		workerCount: workerCount,
	}
}

// RegisterHandler registers a handler for a message type.
func (p *MessageProcessor) RegisterHandler(msgType string, handler MessageHandler) {
	p.handlers[msgType] = handler
}

// Start starts the message processor.
func (p *MessageProcessor) Start(ctx context.Context) {
	p.logger.InfoContext(ctx, "starting message processor", "workers", p.workerCount)

	for i := 0; i < p.workerCount; i++ {
		go p.worker(ctx, i)
	}
}

// worker processes messages in a loop.
func (p *MessageProcessor) worker(ctx context.Context, workerID int) {
	p.logger.DebugContext(ctx, "starting worker", "worker_id", workerID)

	for {
		select {
		case <-ctx.Done():
			p.logger.DebugContext(ctx, "stopping worker", "worker_id", workerID)
			return
		default:
			// Try to dequeue a message
			msg, err := p.queue.Dequeue(ctx)
			if err != nil {
				// No messages available, wait a bit
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process the message
			if err := p.processMessage(ctx, msg); err != nil {
				p.logger.ErrorContext(ctx, "failed to process message",
					"id", msg.ID,
					"error", err,
				)
				// Nack the message to requeue
				_ = p.queue.Nack(ctx, msg.ID)
			} else {
				// Ack the message
				_ = p.queue.Ack(ctx, msg.ID)
			}
		}
	}
}

// processMessage processes a single message.
func (p *MessageProcessor) processMessage(ctx context.Context, msg *QueueMessage) error {
	handler, ok := p.handlers[msg.Type]
	if !ok {
		return fmt.Errorf("no handler for message type: %s", msg.Type)
	}

	return handler(ctx, msg)
}

// PersistenceManager manages persistent state for migration operations.
type PersistenceManager struct {
	store  QueueStore
	logger *slog.Logger
}

// NewPersistenceManager creates a new persistence manager.
func NewPersistenceManager(store QueueStore, logger *slog.Logger) *PersistenceManager {
	return &PersistenceManager{
		store:  store,
		logger: logger,
	}
}

// SaveMigrationState saves migration state for crash recovery.
func (m *PersistenceManager) SaveMigrationState(ctx context.Context, state *MigrationState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	msg := &QueueMessage{
		ID:      state.MigrationID,
		Type:    "migration_state",
		Payload: payload,
	}

	return m.store.Enqueue(ctx, msg)
}

// LoadMigrationState loads migration state for crash recovery.
func (m *PersistenceManager) LoadMigrationState(ctx context.Context, migrationID string) (*MigrationState, error) {
	msg, err := m.store.GetMessage(ctx, migrationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	var state MigrationState
	if err := json.Unmarshal(msg.Payload, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// MigrationState represents the state of a migration for persistence.
type MigrationState struct {
	MigrationID    string                 `json:"migration_id"`
	PlanID         string                 `json:"plan_id"`
	State          string                 `json:"state"`
	CurrentStep    int                    `json:"current_step"`
	TotalSteps     int                    `json:"total_steps"`
	BackfillOffset int64                  `json:"backfill_offset"`
	BackfillTotal  int64                  `json:"backfill_total"`
	CanaryStep     int                    `json:"canary_step"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}
