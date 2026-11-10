package site

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/kradalby/asparges/db/dbsqlc"
)

const indexNowKeySetting = "indexnow_key"

// IndexNowKey is the site's IndexNow key, made on first use and kept: search
// engines tie the site's pings to it, so it must survive restarts.
func (s *Store) IndexNowKey(ctx context.Context) (string, error) {
	// 32 hex characters, inside IndexNow's 8–128 and its hex-only reading.
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails

	q := s.db.Queries()

	err := q.InsertSettingIfMissing(ctx, dbsqlc.InsertSettingIfMissingParams{Name: indexNowKeySetting, Value: hex.EncodeToString(b)})
	if err != nil {
		return "", fmt.Errorf("making indexnow key: %w", err)
	}

	key, err := q.GetSetting(ctx, indexNowKeySetting)
	if err != nil {
		return "", fmt.Errorf("reading indexnow key: %w", err)
	}

	return key, nil
}
