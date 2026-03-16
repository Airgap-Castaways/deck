package store

import (
	"fmt"
	"strings"

	"github.com/taedi90/deck/internal/fsutil"
)

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("store root is empty")
	}
	siteRoot, err := fsutil.NewSiteRoot(root)
	if err != nil {
		return nil, fmt.Errorf("resolve store root: %w", err)
	}
	return &Store{root: siteRoot}, nil
}
