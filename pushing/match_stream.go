package pushing

import (
	"github.com/iosis/exchange-api/matching"
	"github.com/iosis/exchange-api/models"
	"github.com/iosis/exchange-api/utils"
	"github.com/shopspring/decimal"
	"time"
)

type MatchStream struct {
	productId string
	sub       *subscription
	bestBid   decimal.Decimal
	bestAsk   decimal.Decimal
	tick24h   *models.Tick
	tick30d   *models.Tick
	logReader matching.LogReader
}

func newMatchStream(productId string, sub *subscription, logReader matching.LogReader) *MatchStream {
	s := &MatchStream{
		productId: productId,
		sub:       sub,
		logReader: logReader,
	}

	s.logReader.RegisterObserver(s)
	return s
}

func (s *MatchStream) Start() {
	// -1 : read from end
	go s.logReader.Run(0, -1)
}

func (s *MatchStream) OnOpenLog(log *matching.OpenLog, offset int64) {
	// do nothing
}

func (s *MatchStream) OnDoneLog(log *matching.DoneLog, offset int64) {
	// do nothing
}

func (s *MatchStream) OnMatchLog(log *matching.MatchLog, offset int64) {
	// push match
	s.sub.publish(ChannelMatch.FormatWithProductId(log.ProductId), &MatchMessage{
		Type:         "match",
		TradeSeq:     log.TradeSeq,
		TradeId:      log.TradeId,
		Sequence:     log.Sequence,
		Time:         log.Time.Format(time.RFC3339),
		ProductId:    log.ProductId,
		Price:        log.Price.String(),
		Side:         log.Side.String(),
		MakerOrderId: utils.I64ToA(log.MakerOrderId),
		TakerOrderId: utils.I64ToA(log.TakerOrderId),
		Size:         log.Size.String(),
	})
}
