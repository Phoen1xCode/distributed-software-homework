package snowflake

import (
	"fmt"
	"sync"
	"time"
)

const (
	epoch          = int64(1704067200000) // 2024-01-01 00:00:00 UTC
	nodeBits       = 10
	sequenceBits   = 12
	nodeMax        = -1 ^ (-1 << nodeBits)
	sequenceMask   = -1 ^ (-1 << sequenceBits)
	nodeShift      = sequenceBits
	timestampShift = nodeBits + sequenceBits
)

type Node struct {
	mu        sync.Mutex
	timestamp int64
	node      int64
	sequence  int64
}

func NewNode(node int64) (*Node, error) {
	if node < 0 || node > nodeMax {
		return nil, fmt.Errorf("node ID must be between 0 and %d", nodeMax)
	}
	return &Node{node: node}, nil
}

func (n *Node) Generate() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixMilli() - epoch

	if now == n.timestamp {
		n.sequence = (n.sequence + 1) & sequenceMask
		if n.sequence == 0 {
			for now <= n.timestamp {
				now = time.Now().UnixMilli() - epoch
			}
		}
	} else {
		n.sequence = 0
	}

	n.timestamp = now
	return now<<timestampShift | n.node<<nodeShift | n.sequence
}

func (n *Node) GenerateString() string {
	return fmt.Sprintf("%d", n.Generate())
}
