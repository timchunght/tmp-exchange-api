package mysql

import "github.com/iosis/exchange-api/models"

func (s *Store) GetConfigs() ([]*models.Config, error) {
	var configs []*models.Config
	err := s.db.Find(&configs).Error
	return configs, err
}
