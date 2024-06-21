package releaseswatcher

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/dhowden/tag"
)

func ReadID3(filepath string) (tag.Metadata, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	tag, err := tag.ReadFrom(file)
	if err != nil {
		return nil, err
	}

	return tag, nil
}

func Scan(root string, filenames chan<- string, counter *atomic.Int32) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".mp3" || ext == ".m4a" {
			filenames <- path
			counter.Add(1)
		}
		return nil
	})
	if err != nil {
		return err
	}
	close(filenames)
	return nil
}
