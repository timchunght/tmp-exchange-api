package service

import (
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/iosis/exchange-api/models"
	"github.com/iosis/exchange-api/models/mysql"
	"github.com/shopspring/decimal"
	"log"
)

func PlaceOrder(userId int64, clientOid string, productId string, orderType models.OrderType, side models.Side,
	size, price, funds decimal.Decimal) (*models.Order, error) {
	product, err := GetProductById(productId)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, errors.New(fmt.Sprintf("product not found: %v", productId))
	}

	if orderType == models.OrderTypeLimit {
		size = size.Round(product.BaseScale)
		if size.LessThan(product.BaseMinSize) {
			return nil, fmt.Errorf("size %v less than base min size %v", size, product.BaseMinSize)
		}
		price = price.Round(product.QuoteScale)
		if price.LessThan(decimal.Zero) {
			return nil, fmt.Errorf("price %v less than 0", price)
		}
		funds = size.Mul(price)
	} else if orderType == models.OrderTypeMarket {
		if side == models.SideBuy {
			size = decimal.Zero
			price = decimal.Zero
			funds = funds.Round(product.QuoteScale)
			if funds.LessThan(product.QuoteMinSize) {
				return nil, fmt.Errorf("funds %v less than quote min size %v", funds, product.QuoteMinSize)
			}
		} else {
			size = size.Round(product.BaseScale)
			if size.LessThan(product.BaseMinSize) {
				return nil, fmt.Errorf("size %v less than base min size %v", size, product.BaseMinSize)
			}
			price = decimal.Zero
			funds = decimal.Zero
		}
	} else {
		return nil, errors.New("unknown order type")
	}

	var holdCurrency string
	var holdSize decimal.Decimal
	if side == models.SideBuy {
		holdCurrency, holdSize = product.QuoteCurrency, funds
	} else {
		holdCurrency, holdSize = product.BaseCurrency, size
	}

	order := &models.Order{
		ClientOid: clientOid,
		UserId:    userId,
		ProductId: product.Id,
		Side:      side,
		Size:      size,
		Funds:     funds,
		Price:     price,
		Status:    models.OrderStatusNew,
		Type:      orderType,
	}

	// tx
	db, err := mysql.SharedStore().BeginTx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Rollback() }()

	err = HoldBalance(db, userId, holdCurrency, holdSize, models.BillTypeTrade)
	if err != nil {
		return nil, err
	}

	err = db.AddOrder(order)
	if err != nil {
		return nil, err
	}

	return order, db.CommitTx()
}

func UpdateOrderStatus(orderId int64, oldStatus, newStatus models.OrderStatus) (bool, error) {
	return mysql.SharedStore().UpdateOrderStatus(orderId, oldStatus, newStatus)
}

func ExecuteFill(orderId int64) error {
	// tx
	db, err := mysql.SharedStore().BeginTx()
	if err != nil {
		return err
	}
	defer func() { _ = db.Rollback() }()

	order, err := db.GetOrderByIdForUpdate(orderId)
	if err != nil {
		return err
	}
	if order == nil {
		return fmt.Errorf("order not found: %v", orderId)
	}
	if order.Status == models.OrderStatusFilled || order.Status == models.OrderStatusCancelled {
		return fmt.Errorf("order status invalid: %v %v", orderId, order.Status)
	}

	product, err := GetProductById(order.ProductId)
	if err != nil {
		return err
	}
	if product == nil {
		return fmt.Errorf("product not found: %v", order.ProductId)
	}

	fills, err := mysql.SharedStore().GetUnsettledFillsByOrderId(orderId)
	if err != nil {
		return err
	}
	if len(fills) == 0 {
		return nil
	}

	var bills []*models.Bill
	for _, fill := range fills {
		fill.Settled = true

		notes := fmt.Sprintf("%v-%v", fill.OrderId, fill.Id)

		if !fill.Done {
			executedValue := fill.Size.Mul(fill.Price)
			order.ExecutedValue = order.ExecutedValue.Add(executedValue)
			order.FilledSize = order.FilledSize.Add(fill.Size)

			if order.Side == models.SideBuy {
				// buy order???incr base
				bill, err := AddDelayBill(db, order.UserId, product.BaseCurrency, fill.Size, decimal.Zero,
					models.BillTypeTrade, notes)
				if err != nil {
					return err
				}
				bills = append(bills, bill)

				// buy order???decr quote
				bill, err = AddDelayBill(db, order.UserId, product.QuoteCurrency, decimal.Zero, executedValue.Neg(),
					models.BillTypeTrade, notes)
				if err != nil {
					return err
				}
				bills = append(bills, bill)

			} else {
				// sell order???decr base
				bill, err := AddDelayBill(db, order.UserId, product.BaseCurrency, decimal.Zero, fill.Size.Neg(),
					models.BillTypeTrade, notes)
				if err != nil {
					return err
				}
				bills = append(bills, bill)

				// sell order???incr quote
				bill, err = AddDelayBill(db, order.UserId, product.QuoteCurrency, executedValue, decimal.Zero,
					models.BillTypeTrade, notes)
				if err != nil {
					return err
				}
				bills = append(bills, bill)
			}

		} else {
			if fill.DoneReason == models.DoneReasonCancelled {
				order.Status = models.OrderStatusCancelled
			} else if fill.DoneReason == models.DoneReasonFilled {
				order.Status = models.OrderStatusFilled
			} else {
				log.Fatalf("unknown done reason: %v", fill.DoneReason)
			}

			if order.Side == models.SideBuy {

				// if it is still buy order, unfreeze reamining funds required
				remainingFunds := order.Funds.Sub(order.ExecutedValue)
				if remainingFunds.GreaterThan(decimal.Zero) {
					bill, err := AddDelayBill(db, order.UserId, product.QuoteCurrency, remainingFunds, remainingFunds.Neg(),
						models.BillTypeTrade, notes)
					if err != nil {
						return err
					}
					bills = append(bills, bill)
				}

			} else {

				// if it is sell order, unfreeze remaining size
				remainingSize := order.Size.Sub(order.FilledSize)
				if remainingSize.GreaterThan(decimal.Zero) {
					bill, err := AddDelayBill(db, order.UserId, product.BaseCurrency, remainingSize, remainingSize.Neg(),
						models.BillTypeTrade, notes)
					if err != nil {
						return err
					}
					bills = append(bills, bill)
				}
			}

			break
		}
	}

	err = db.UpdateOrder(order)
	if err != nil {
		return err
	}

	for _, fill := range fills {
		err = db.UpdateFill(fill)
		if err != nil {
			return err
		}
	}

	return db.CommitTx()
}

func GetOrderById(orderId int64) (*models.Order, error) {
	return mysql.SharedStore().GetOrderById(orderId)
}

func GetOrderByClientOid(userId int64, clientOid string) (*models.Order, error) {
	return mysql.SharedStore().GetOrderByClientOid(userId, clientOid)
}

func GetOrdersByUserId(userId int64, statuses []models.OrderStatus, side *models.Side, productId string,
	beforeId, afterId int64, limit int) ([]*models.Order, error) {
	return mysql.SharedStore().GetOrdersByUserId(userId, statuses, side, productId, beforeId, afterId, limit)
}
