package mimage

import (
	"fmt"
	"os"
	"path/filepath"
)

// Option is something that can be configured on an Mimage object
type Option func(*Mimage) error

// ChunkSize sets non-default chunksize
func ChunkSize(i int) Option {
	return func(m *Mimage) error {
		if i <= 0 {
			return fmt.Errorf("chunksize must be greater than zero, given %d", i)
		}
		m.chunkSize = i
		return nil
	}
}

// Directory configures a new Mimage to use an existing directory.
// If not found, this will create the directory(/ies) as needed.
//
// If this isn't given a new Mimage will use a random temp directory.
//
// Nb. we do not expect to find Mimage metadata files / image chunks
// in the directory if it exists already.
func Directory(s string) Option {
	return func(m *Mimage) error {
		info, err := os.Stat(s)
		if os.IsNotExist(err) {
			m.root = s
			return os.MkdirAll(s, 0640)
		} else if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("given path %s is not a directory", s)
		}

		_, err = os.Stat(filepath.Join(s, metafile))
		if err == nil {
			return fmt.Errorf("expected path %s not to contain %s", s, metafile)
		}
		m.root = s
		return nil
	}
}

// OperationRoutines sets how many routines are available to
// process image chunks during an operation's Do() call.
// This __roughly__ equates to how many image chunks we have
// in memory at any one time.
func OperationRoutines(i int) Option {
	return func(m *Mimage) error {
		if i <= 0 {
			i = 1
		}
		m.routines = i
		return nil
	}
}
