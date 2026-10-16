package commands

import (
	"maps"

	"github.com/aaron-vaz/skelly/internal/config"
)

type mockStore struct {
	cfg     config.Config
	loadErr error
	saveErr error
	saves   int
}

func (m *mockStore) Load() (*config.Config, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	// Hand out a copy so only Save can change what is stored
	return &config.Config{Sources: maps.Clone(m.cfg.Sources)}, nil
}

func (m *mockStore) Save(cfg *config.Config) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saves++
	m.cfg = config.Config{Sources: maps.Clone(cfg.Sources)}
	return nil
}
