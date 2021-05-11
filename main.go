package main

import (
	"github.com/iosis/exchange-api/conf"
	"github.com/iosis/exchange-api/matching"
	"github.com/iosis/exchange-api/models"
	"github.com/iosis/exchange-api/pushing"
	"github.com/iosis/exchange-api/rest"
	"github.com/iosis/exchange-api/service"
	"github.com/iosis/exchange-api/worker"
	"github.com/prometheus/common/log"
	"net/http"
	_ "net/http/pprof"
)

func main() {
	gbeConfig := conf.GetConfig()

	go func() {
		log.Info(http.ListenAndServe("localhost:6060", nil))
	}()

	go models.NewBinLogStream().Start()

	matching.StartEngine()

	pushing.StartServer()

	worker.NewFillExecutor().Start()
	worker.NewBillExecutor().Start()
	products, err := service.GetProducts()
	if err != nil {
		panic(err)
	}
	for _, product := range products {
		worker.NewTickMaker(product.Id, matching.NewKafkaLogReader("tickMaker", product.Id, gbeConfig.Kafka.Brokers)).Start()
		worker.NewFillMaker(matching.NewKafkaLogReader("fillMaker", product.Id, gbeConfig.Kafka.Brokers)).Start()
		worker.NewTradeMaker(matching.NewKafkaLogReader("tradeMaker", product.Id, gbeConfig.Kafka.Brokers)).Start()
	}

	rest.StartServer()

	select {}
}
