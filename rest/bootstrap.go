package rest

import (
	"github.com/iosis/exchange-api/conf"
	"github.com/siddontang/go-log/log"
)

func StartServer() {
	gbeConfig := conf.GetConfig()

	httpServer := NewHttpServer(gbeConfig.RestServer.Addr)
	go httpServer.Start()

	log.Info("rest server ok")
}
