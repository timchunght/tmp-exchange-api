package models

import (
	"fmt"
	"github.com/shopspring/decimal"
	"time"
)

// Use for representing Buy, Sell side
type Side string

func NewSideFromString(s string) (*Side, error) {
	side := Side(s)
	switch side {
	case SideBuy:
	case SideSell:
	default:
		return nil, fmt.Errorf("invalid side: %v", s)
	}
	return &side, nil
}

func (s Side) Opposite() Side {
	if s == SideBuy {
		return SideSell
	}
	return SideBuy
}

func (s Side) String() string {
	return string(s)
}

type OrderType string

func (t OrderType) String() string {
	return string(t)
}

type OrderStatus string

func NewOrderStatusFromString(s string) (*OrderStatus, error) {
	status := OrderStatus(s)
	switch status {
	case OrderStatusNew:
	case OrderStatusOpen:
	case OrderStatusCancelling:
	case OrderStatusCancelled:
	case OrderStatusFilled:
	default:
		return nil, fmt.Errorf("invalid status: %v", s)
	}
	return &status, nil
}

func (t OrderStatus) String() string {
	return string(t)
}

type BillType string

// Reason for fill completion
type DoneReason string

type TransactionStatus string

const (
	OrderTypeLimit  = OrderType("limit")
	OrderTypeMarket = OrderType("market")

	SideBuy  = Side("buy")
	SideSell = Side("sell")

	// Order initial state
	OrderStatusNew = OrderStatus("new")

	// Added to orderBook
	OrderStatusOpen = OrderStatus("open")

	// In progress state, request for cancelling order
	OrderStatusCancelling = OrderStatus("cancelling")

	// Order has been canclled, partially filled order is cancelled
	OrderStatusCancelled = OrderStatus("cancelled")

	// Order completed filled
	OrderStatusFilled = OrderStatus("filled")

	BillTypeTrade = BillType("trade")

	DoneReasonFilled    = DoneReason("filled")
	DoneReasonCancelled = DoneReason("cancelled")

	TransactionStatusPending   = TransactionStatus("pending")
	TransactionStatusCompleted = TransactionStatus("completed")
)

type User struct {
	Id           int64 `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	UserId       int64
	Email        string
	PasswordHash string
}

type Account struct {
	Id        int64 `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	CreatedAt time.Time
	UpdatedAt time.Time
	UserId    int64           `gorm:"column:user_id;unique_index:idx_uid_currency"`
	Currency  string          `gorm:"column:currency;unique_index:idx_uid_currency"`
	Hold      decimal.Decimal `gorm:"column:hold" sql:"type:decimal(32,16);"`
	Available decimal.Decimal `gorm:"column:available" sql:"type:decimal(32,16);"`
}

type Bill struct {
	Id        int64 `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	CreatedAt time.Time
	UpdatedAt time.Time
	UserId    int64
	Currency  string
	Available decimal.Decimal `sql:"type:decimal(32,16);"`
	Hold      decimal.Decimal `sql:"type:decimal(32,16);"`
	Type      BillType
	Settled   bool
	Notes     string
}

type Product struct {
	Id             string `gorm:"column:id;primary_key"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	BaseCurrency   string
	QuoteCurrency  string
	BaseMinSize    decimal.Decimal `sql:"type:decimal(32,16);"`
	BaseMaxSize    decimal.Decimal `sql:"type:decimal(32,16);"`
	QuoteMinSize   decimal.Decimal `sql:"type:decimal(32,16);"`
	QuoteMaxSize   decimal.Decimal `sql:"type:decimal(32,16);"`
	BaseScale      int32
	QuoteScale     int32
	QuoteIncrement float64
}

type Order struct {
	Id            int64 `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ProductId     string
	UserId        int64
	ClientOid     string
	Size          decimal.Decimal `sql:"type:decimal(32,16);"`
	Funds         decimal.Decimal `sql:"type:decimal(32,16);"`
	FilledSize    decimal.Decimal `sql:"type:decimal(32,16);"`
	ExecutedValue decimal.Decimal `sql:"type:decimal(32,16);"`
	Price         decimal.Decimal `sql:"type:decimal(32,16);"`
	FillFees      decimal.Decimal `sql:"type:decimal(32,16);"`
	Type          OrderType
	Side          Side
	TimeInForce   string
	Status        OrderStatus
	Settled       bool
}

type Fill struct {
	Id         int64 `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	TradeId    string `gorm:"column:trade_id"`
	TradeSeq   int64  `gorm:"column:trade_seq"`
	OrderId    int64  `gorm:"unique_index:o_m"`
	MessageSeq int64  `gorm:"unique_index:o_m"`
	ProductId  string
	Size       decimal.Decimal `sql:"type:decimal(32,16);"`
	Price      decimal.Decimal `sql:"type:decimal(32,16);"`
	Funds      decimal.Decimal `sql:"type:decimal(32,16);"`
	Fee        decimal.Decimal `sql:"type:decimal(32,16);"`
	Liquidity  string
	Settled    bool
	Side       Side
	Done       bool
	DoneReason DoneReason
	LogOffset  int64
	LogSeq     int64
}

type Trade struct {
	Id           string `gorm:"column:id;primary_key"`
	TradeSeq     int64  `gorm:"column:trade_seq"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ProductId    string
	TakerOrderId int64
	MakerOrderId int64
	Price        decimal.Decimal `sql:"type:decimal(32,16);"`
	Size         decimal.Decimal `sql:"type:decimal(32,16);"`
	Side         Side
	Time         time.Time
	LogOffset    int64
	LogSeq       int64
}

type Tick struct {
	Id          int64 `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ProductId   string          `gorm:"unique_index:p_g_t"`
	Granularity int64           `gorm:"unique_index:p_g_t"`
	Time        int64           `gorm:"unique_index:p_g_t"`
	Open        decimal.Decimal `sql:"type:decimal(32,16);"`
	High        decimal.Decimal `sql:"type:decimal(32,16);"`
	Low         decimal.Decimal `sql:"type:decimal(32,16);"`
	Close       decimal.Decimal `sql:"type:decimal(32,16);"`
	Volume      decimal.Decimal `sql:"type:decimal(32,16);"`
	LogOffset   int64
	LogSeq      int64
}

type Config struct {
	Id        int64 `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Key       string
	Value     string
}

type Transaction struct {
	Id          int64 `gorm:"column:id;primary_key;AUTO_INCREMENT"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	UserId      int64
	Currency    string
	BlockNum    int
	ConfirmNum  int
	Status      TransactionStatus
	FromAddress string
	ToAddress   string
	Note        string
	TxId        string
}
