package storecassandra

import (
	"github.com/cloudfoundry/hm9000/storeadapter"
	"tux21b.org/v1/gocql"
)

func (s *StoreCassandra) SaveMetric(metric string, value int) error {
	return s.session.Query(`INSERT INTO Metrics (key, value) VALUES (?, ?)`, metric, value).Exec()
}

func (s *StoreCassandra) GetMetric(metric string) (int, error) {
	var value int
	err := s.session.Query(`SELECT value FROM Metrics WHERE key=?`, metric).Scan(&value)

	if err == gocql.ErrNotFound {
		return -1, storeadapter.ErrorKeyNotFound
	}

	if err != nil {
		return 0, err
	}

	return value, nil
}
