package loader

import (
	"os"
	"path/filepath"

	"github.com/avijeet7/protomock/internal/models"
)

// LoadMocks walks the provided root directory and loads all valid proto+stub combinations.
func LoadMocks(root string) ([]models.Route, error) {
	var routes []models.Route

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isProtoFile(d.Name()) {
			return err
		}
		newRoutes, err := parseProtoAndStubs(path)
		if err == nil {
			routes = append(routes, newRoutes...)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return routes, nil
}
