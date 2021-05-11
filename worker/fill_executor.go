package worker

import (
	"encoding/json"
	"github.com/go-redis/redis"
	lru "github.com/hashicorp/golang-lru"
	"github.com/iosis/exchange-api/conf"
	"github.com/iosis/exchange-api/models"
	"github.com/iosis/exchange-api/service"
	"github.com/siddontang/go-log/log"
	"time"
)

const fillWorkerNum = 10

type FillExecutor struct {

	// used for receiving fill after internal sharding,
	// use orderId for sharding，can lower possibility of lock contention，
	workerChs [fillWorkerNum]chan *models.Fill
}

func NewFillExecutor() *FillExecutor {
	f := &FillExecutor{
		workerChs: [fillWorkerNum]chan *models.Fill{},
	}

	// initialze the same number of routine as fillWorkerNum
	// each routine handles one chan
	for i := 0; i < fillWorkerNum; i++ {
		f.workerChs[i] = make(chan *models.Fill, 512)
		go func(idx int) {
			settledOrderCache, err := lru.New(1000)
			if err != nil {
				panic(err)
			}

			for {
				select {
				case fill := <-f.workerChs[idx]:
					if settledOrderCache.Contains(fill.OrderId) {
						continue
					}

					order, err := service.GetOrderById(fill.OrderId)
					if err != nil {
						log.Error(err)
					}
					if order == nil {
						log.Warnf("order not found: %v", fill.OrderId)
						continue
					}
					if order.Status == models.OrderStatusCancelled || order.Status == models.OrderStatusFilled {
						settledOrderCache.Add(order.Id, struct{}{})
						continue
					}

					err = service.ExecuteFill(fill.OrderId)
					if err != nil {
						log.Error(err)
					}
				}
			}
		}(i)
	}

	return f
}

func (s *FillExecutor) Start() {
	go s.runInspector()
	go s.runMqListener()
}

// Listens for message notification from queue
func (s *FillExecutor) runMqListener() {
	gbeConfig := conf.GetConfig()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     gbeConfig.Redis.Addr,
		Password: gbeConfig.Redis.Password,
		DB:       0,
	})

	for {
		ret := redisClient.BRPop(time.Second*1000, models.TopicFill)
		if ret.Err() != nil {
			log.Error(ret.Err())
			continue
		}

		var fill models.Fill
		err := json.Unmarshal([]byte(ret.Val()[1]), &fill)
		if err != nil {
			log.Error(err)
			continue
		}

		// use orderId for sharding, same orderId will be sent to the same chan
		s.workerChs[fill.OrderId%fillWorkerNum] <- &fill
	}
}

// Periodically poll database
func (s *FillExecutor) runInspector() {
	for {
		select {
		case <-time.After(1 * time.Second):
			fills, err := service.GetUnsettledFills(1000)
			if err != nil {
				log.Error(err)
				continue
			}

			for _, fill := range fills {
				s.workerChs[fill.OrderId%fillWorkerNum] <- fill
			}
		}
	}
}
