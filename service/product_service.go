package service

import (
	"github.com/iosis/exchange-api/models"
	"github.com/iosis/exchange-api/models/mysql"
)

func GetProductById(id string) (*models.Product, error) {
	return mysql.SharedStore().GetProductById(id)
}

func GetProducts() ([]*models.Product, error) {
	return mysql.SharedStore().GetProducts()
}
