package matching

import (
	"github.com/iosis/exchange-api/models"
)

// Use for reading orders from matching engine
// setting offset and fetch orders from offset
type OrderReader interface {

	// Set the offset for where to start order fetching
	SetOffset(offset int64) error

	// Fetch orders
	FetchOrder() (offset int64, order *models.Order, err error)
}

// For Log storage
type LogStore interface {

	// Persist log
	Store(logs []interface{}) error
}

// Use LogReader mode to get matching log
type LogReader interface {

	// Get current productID
	GetProductId() string

	// Register Log Observer
	RegisterObserver(observer LogObserver)

	// Start log reading, retrieved log will be sent to observer callbcak
	Run(seq, offset int64)
}

// Matching engine log observer
type LogObserver interface {
	// callback for when OpenLog succeeds
	OnOpenLog(log *OpenLog, offset int64)

	// callback for when MatchLog succeeds
	OnMatchLog(log *MatchLog, offset int64)

	// callback for when DoneLog succeeds
	OnDoneLog(log *DoneLog, offset int64)
}

// 用于保存撮合引擎的快照
type SnapshotStore interface {

	// Persist snapshot
	Store(snapshot *Snapshot) error

	// Get latest snapshot
	GetLatest() (*Snapshot, error)
}
