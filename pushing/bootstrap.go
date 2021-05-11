package pushing

import (
	"github.com/iosis/exchange-api/conf"
	"github.com/iosis/exchange-api/matching"
	"github.com/iosis/exchange-api/service"
	"github.com/siddontang/go-log/log"
)

func StartServer() {
	gbeConfig := conf.GetConfig()

	sub := newSubscription()

	newRedisStream(sub).Start()

	products, err := service.GetProducts()
	if err != nil {
		panic(err)
	}
	for _, product := range products {
		newTickerStream(product.Id, sub, matching.NewKafkaLogReader("tickerStream", product.Id, gbeConfig.Kafka.Brokers)).Start()
		newMatchStream(product.Id, sub, matching.NewKafkaLogReader("matchStream", product.Id, gbeConfig.Kafka.Brokers)).Start()
		newOrderBookStream(product.Id, sub, matching.NewKafkaLogReader("orderBookStream", product.Id, gbeConfig.Kafka.Brokers)).Start()
	}

	go NewServer(gbeConfig.PushServer.Addr, gbeConfig.PushServer.Path, sub).Run()

	log.Info("websocket server ok")
}
