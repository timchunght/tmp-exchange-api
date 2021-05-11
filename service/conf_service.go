package service

import (
	"github.com/iosis/exchange-api/models"
	"github.com/iosis/exchange-api/models/mysql"
)

func GetConfigs() ([]*models.Config, error) {
	return mysql.SharedStore().GetConfigs()
}
