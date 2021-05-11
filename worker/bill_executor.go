package worker

import (
	"encoding/json"
	"github.com/go-redis/redis"
	"github.com/iosis/exchange-api/conf"
	"github.com/iosis/exchange-api/models"
	"github.com/iosis/exchange-api/service"
	"github.com/siddontang/go-log/log"
	"time"
)

type BillExecutor struct {
	workerChs [fillWorkerNum]chan *models.Bill
}

func NewBillExecutor() *BillExecutor {
	f := &BillExecutor{
		workerChs: [fillWorkerNum]chan *models.Bill{},
	}

	// initialize fillWorkersNum with the same num as routine，one routine each chan
	for i := 0; i < fillWorkerNum; i++ {
		f.workerChs[i] = make(chan *models.Bill, 256)
		go func(idx int) {
			for {
				select {
				case bill := <-f.workerChs[idx]:
					err := service.ExecuteBill(bill.UserId, bill.Currency)
					if err != nil {
						log.Error(err)
					}
				}
			}
		}(i)
	}
	return f
}

func (s *BillExecutor) Start() {
	go s.runMqListener()
	go s.runInspector()
}

func (s *BillExecutor) runMqListener() {
	gbeConfig := conf.GetConfig()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     gbeConfig.Redis.Addr,
		Password: gbeConfig.Redis.Password,
		DB:       0,
	})

	for {
		ret := redisClient.BRPop(time.Second*1000, models.TopicBill)
		if ret.Err() != nil {
			log.Error(ret.Err())
			continue
		}

		var bill models.Bill
		err := json.Unmarshal([]byte(ret.Val()[1]), &bill)
		if err != nil {
			panic(ret.Err())
		}

		// use userId for sharding
		s.workerChs[bill.UserId%fillWorkerNum] <- &bill
	}
}

func (s *BillExecutor) runInspector() {
	for {
		select {
		case <-time.After(1 * time.Second):
			bills, err := service.GetUnsettledBills()
			if err != nil {
				log.Error(err)
				continue
			}

			for _, bill := range bills {
				s.workerChs[bill.UserId%fillWorkerNum] <- bill
			}
		}
	}
}
