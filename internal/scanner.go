package releaseswatcher

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/bogem/id3v2"
)

type Tags struct {
}

func ReadID3(filepath string) (*id3v2.Tag, error) {
	tag, err := id3v2.Open(filepath, id3v2.Options{Parse: true})
	if err != nil {
		return nil, err
	}
	defer tag.Close()

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
