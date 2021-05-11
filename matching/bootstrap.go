package matching

import (
	"github.com/iosis/exchange-api/conf"
	"github.com/iosis/exchange-api/service"
	"github.com/siddontang/go-log/log"
	"os/signal"
	"syscall"
)

func StartEngine() {
	gbeConfig := conf.GetConfig()

	signal.Ignore(syscall.SIGCHLD)

	products, err := service.GetProducts()
	if err != nil {
		panic(err)
	}
	for _, product := range products {
		orderReader := NewKafkaOrderReader(product.Id, gbeConfig.Kafka.Brokers)
		snapshotStore := NewRedisSnapshotStore(product.Id)
		logStore := NewKafkaLogStore(product.Id, gbeConfig.Kafka.Brokers)
		matchEngine := NewEngine(product, orderReader, logStore, snapshotStore)
		matchEngine.Start()
	}

	log.Info("match engine ok")
}
