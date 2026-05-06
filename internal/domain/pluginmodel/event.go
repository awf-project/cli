package pluginmodel

import (
	"crypto/rand"
	"fmt"
	"time"
)

type DomainEvent struct {
	ID               string
	Type             string
	Timestamp        time.Time
	Source           string
	Metadata         map[string]string
	Payload          []byte
	PropagationDepth int
}

func generateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func NewDomainEvent(eventType, source string, metadata map[string]string, payload []byte) *DomainEvent {
	return &DomainEvent{
		ID:        generateUUID(),
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Metadata:  metadata,
		Payload:   payload,
	}
}
