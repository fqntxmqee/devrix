package kernel

import (
	"fmt"
	"time"
)

// NewMessageID generates a unique outbound message id.
func NewMessageID() string {
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%10000)
}
