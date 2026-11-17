package library

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/lizhian/viva-agents-manager/internal/domain"
)

const sourceMetadata = ".source.json"

type sourceAddress struct {
	Address string `json:"address"`
}

// writeSourceAddress is called on staging directories before publishing them.
func writeSourceAddress(dir, address string) error {
	if domain.ValidRepo(address) {
		return nil
	}
	data, err := json.Marshal(sourceAddress{Address: address})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, sourceMetadata), data, 0o600); err != nil {
		return fmt.Errorf("保存来源地址失败: %w", err)
	}
	return nil
}

// sourceAddress reads URL identity from disk, even with a missing/rebuilt index.
// Invalid metadata leaves the skills visible as local files, never updates them
// from an unverified address.
func (l *Library) sourceAddress(name string) string {
	if repo := domain.RepoOf(name); repo != "" {
		return repo
	}
	data, err := os.ReadFile(filepath.Join(l.config.Root, name, sourceMetadata))
	if os.IsNotExist(err) {
		return ""
	}
	var metadata sourceAddress
	if err == nil {
		err = json.Unmarshal(data, &metadata)
	}
	if err == nil && domain.ValidInstallSource(metadata.Address) && !domain.ValidRepo(metadata.Address) && domain.MatchesSourceName(name, metadata.Address) {
		return metadata.Address
	}
	log.Printf("library: 来源 %s 的地址元数据不可用: %v", name, err)
	return ""
}
