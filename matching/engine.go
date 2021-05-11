package matching

/*
#include <stdlib.h>
#include <unistd.h>
*/
import "C"
import (
	"github.com/iosis/exchange-api/models"
	logger "github.com/siddontang/go-log/log"
	"time"
)

type Engine struct {
	// productId is the only unique ID for engine，
	// each product has one corresponding engine
	productId string

	// engine's associated orderBook，corresponds to product，
	// requires snapshot and can recover from snapshot
	OrderBook *orderBook

	// used for reading order
	orderReader OrderReader

	// read order's initial offset，
	// this value will be restored from snapshot when it first starts
	appliedOrderOffset int64

	// 读取的order会写入chan，写入order的同时需要携带该order的offset
	orderCh chan *offsetOrder

	// 用于保存orderBook log
	logStore LogStore

	// log写入队列，所有待写入的log需要进入该chan等待
	logCh chan Log

	// 发起snapshot请求，需要携带最后一次snapshot的offset
	snapshotReqCh chan *Snapshot

	// snapshot已经完全准备好，需要确保snapshot之前的所有数据都已经提交
	snapshotApproveReqCh chan *Snapshot

	// snapshot数据准备好并且snapshot之前的所有数据都已经提交
	snapshotCh chan *Snapshot

	// 持久化snapshot的存储方式，应该支持多种方式，如本地磁盘，redis等
	snapshotStore SnapshotStore

	lastSnapOrderOffset int64
	committedLogSeq     int64
}

// 快照是engine在某一时候的一致性内存状态
type Snapshot struct {
	OrderBookSnapshot orderBookSnapshot
	OrderOffset       int64
}

type offsetOrder struct {
	Offset int64
	Order  *models.Order
}

func NewEngine(product *models.Product, orderReader OrderReader, logStore LogStore, snapshotStore SnapshotStore) *Engine {
	e := &Engine{
		productId:            product.Id,
		OrderBook:            NewOrderBook(product),
		logCh:                make(chan Log, 10000),
		orderCh:              make(chan *offsetOrder, 10000),
		snapshotReqCh:        make(chan *Snapshot, 32),
		snapshotApproveReqCh: make(chan *Snapshot, 32),
		snapshotCh:           make(chan *Snapshot, 32),
		snapshotStore:        snapshotStore,
		orderReader:          orderReader,
		logStore:             logStore,
	}

	// 获取最新的snapshot，并使用snapshot进行恢复
	snapshot, err := snapshotStore.GetLatest()
	if err != nil {
		logger.Fatalf("get latest snapshot error: %v", err)
	}
	if snapshot != nil {
		e.restore(snapshot)
	}
	return e
}

func (e *Engine) Start() {
	go e.runFetcher()
	go e.runApplier()
	go e.runCommitter()
	go e.runSnapshots()
}

// 负责不断的拉取order，写入chan
func (e *Engine) runFetcher() {
	var offset = e.appliedOrderOffset
	if offset > 0 {
		offset = offset + 1
	}
	err := e.orderReader.SetOffset(offset)
	if err != nil {
		logger.Fatalf("set order reader offset error: %v", err)
	}

	for {
		offset, order, err := e.orderReader.FetchOrder()
		if err != nil {
			logger.Error(err)
			continue
		}
		e.orderCh <- &offsetOrder{offset, order}
	}
}

// 从本地队列获取order，执行orderBook操作，同时要响应snapshot请求
func (e *Engine) runApplier() {
	var orderOffset int64

	for {
		select {
		case offsetOrder := <-e.orderCh:
			// put or cancel order
			var logs []Log
			if offsetOrder.Order.Status == models.OrderStatusCancelling {
				logs = e.OrderBook.CancelOrder(offsetOrder.Order)
			} else {
				logs = e.OrderBook.ApplyOrder(offsetOrder.Order)
			}

			// 将orderBook产生的log写入chan进行持久化
			for _, log := range logs {
				e.logCh <- log
			}

			// 记录订单的offset用于判断是否需要进行快照
			orderOffset = offsetOrder.Offset
			e.appliedOrderOffset = offsetOrder.Offset

		case snapshot := <-e.snapshotReqCh:
			// 接收到快照请求，判断是否真的需要执行快照
			delta := orderOffset - snapshot.OrderOffset
			if delta <= 1000 {
				continue
			}

			logger.Infof("should take snapshot: %v %v-[%v]-%v->",
				e.productId, snapshot.OrderOffset, delta, orderOffset)

			e.saveSnapshotBackground()

			// 执行快照，并将快照数据写入批准chan
			snapshot.OrderBookSnapshot = e.OrderBook.Snapshot()
			snapshot.OrderOffset = orderOffset
			e.snapshotApproveReqCh <- snapshot
		}
	}
}

// persists log from orderBook
// simultaneously requires corresponding snapshot confirmation同时需要响应snapshot审批
func (e *Engine) runCommitter() {
	var seq = e.OrderBook.logSeq
	var pending *Snapshot = nil
	var logs []interface{}

	for {
		select {
		case log := <-e.logCh:
			// discard duplicate log
			if log.GetSeq() <= seq {
				logger.Infof("discard log seq=%v", seq)
				continue
			}

			seq = log.GetSeq()
			logs = append(logs, log)

			// chan is not empty and buffer is not full, continue read.
			if len(e.logCh) > 0 && len(logs) < 100 {
				continue
			}

			// store log, clean buffer
			err := e.logStore.Store(logs)
			if err != nil {
				panic(err)
			}
			logs = nil
			e.committedLogSeq = seq

			// approve pending snapshot
			if pending != nil && seq >= pending.OrderBookSnapshot.LogSeq {
				e.snapshotCh <- pending
				pending = nil
			}

		case snapshot := <-e.snapshotApproveReqCh:
			// written seq is >= snapshot seq, approve snapshot request
			if seq >= snapshot.OrderBookSnapshot.LogSeq {
				e.snapshotCh <- snapshot
				pending = nil
				continue
			}

			// 当前还有未批准的snapshot，但是又有新的snapshot请求，丢弃旧的请求
			if pending != nil {
				logger.Infof("discard snapshot request (seq=%v), new one (seq=%v) received",
					pending.OrderBookSnapshot.LogSeq, snapshot.OrderBookSnapshot.LogSeq)
			}
			pending = snapshot
		}
	}
}

// periodically run snapshot request
// also handles persistence for confirmed snapshot
func (e *Engine) runSnapshots() {
	// final shapshot's order orderOffset
	orderOffset := e.appliedOrderOffset

	for {
		select {
		case <-time.After(30 * time.Second):
			// make a new snapshot request
			e.snapshotReqCh <- &Snapshot{
				OrderOffset: orderOffset,
			}

		case snapshot := <-e.snapshotCh:
			// store snapshot
			err := e.snapshotStore.Store(snapshot)
			if err != nil {
				logger.Warnf("store snapshot failed: %v", err)
				continue
			}
			logger.Infof("new snapshot stored :product=%v OrderOffset=%v LogSeq=%v",
				e.productId, snapshot.OrderOffset, snapshot.OrderBookSnapshot.LogSeq)

			// update offset for next snapshot request
			orderOffset = snapshot.OrderOffset
		}
	}
}

func (e *Engine) snapshot() *Snapshot {
	return &Snapshot{
		OrderBookSnapshot: e.OrderBook.Snapshot(),
		OrderOffset:       e.appliedOrderOffset,
	}
}

func (e *Engine) restore(snapshot *Snapshot) {
	e.appliedOrderOffset = snapshot.OrderOffset
	e.OrderBook.Restore(&snapshot.OrderBookSnapshot)
}

func (e *Engine) saveSnapshotBackground() {
	pid := C.fork()
	if pid < 0 {
		logger.Warn("fork error")
	} else if pid == 0 {
		//logger.Infof("[pid=%v] taking snapshot", os.Getpid())
		/*snap := e.snapshot()
		if snap.OrderBookSnapshot.LogSeq < e.committedLogSeq {
			//logger.Warnf("[pid=%v] snapshot logSeq less than committed logSeq", os.Getpid())
		}*/

		/*err := e.snapshotStore.Store(snap)
		if err != nil {
			logger.Warnf("store snapshot failed: %v", err)
			return
		}*/
		//logger.Infof("[pid=%v] new snapshot stored :product=%v OrderOffset=%v LogSeq=%v pid=%v",
		//	os.Getpid(), e.productId, snap.OrderOffset, snap.OrderBookSnapshot.LogSeq, os.Getpid())

		// exit child process
		//os.Exit(0)
		C.exit(0)

	} else {
		e.lastSnapOrderOffset = e.appliedOrderOffset
	}
}
