//go:build integration || test

package events

import (
	"context"
	"sync"
)

type PublishedEvent struct {
	EventType     string
	CorrelationID string
	Payload       any
}

type MockPublisher struct {
	mu              sync.Mutex
	PublishedEvents []PublishedEvent
}

func NewMockPublisher() *MockPublisher {
	return &MockPublisher{PublishedEvents: make([]PublishedEvent, 0)}
}

func (m *MockPublisher) Publish(ctx context.Context, eventType, correlationID string, payload any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PublishedEvents = append(m.PublishedEvents, PublishedEvent{
		EventType:     eventType,
		CorrelationID: correlationID,
		Payload:       payload,
	})
	return nil
}

func (m *MockPublisher) Events() []PublishedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]PublishedEvent, len(m.PublishedEvents))
	copy(copied, m.PublishedEvents)
	return copied
}

func (m *MockPublisher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PublishedEvents = make([]PublishedEvent, 0)
}
