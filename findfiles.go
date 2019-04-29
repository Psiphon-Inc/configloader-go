/*
 * BSD 3-Clause License
 * Copyright (c) 2019, Psiphon Inc.
 * All rights reserved.
 */

package configloader

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// Assigning to a variable to assist with testing (to force errors)
var osOpen = os.Open

// FileLocation is the name of a (potential) config file, and the places where it should
// be looked for.
type FileLocation struct {
	// The filename will be searched for relative to each of the search paths. If one of
	// the search paths is "", then Filename will also be searched for as an absolute path.
	Filename string

	// The file will be search for through the SearchPaths. These are in order -- the
	// search will stop on the first match.
	SearchPaths []string
}

// FindFiles assists with figuring out which config files should be used.
//
// fileLocations is the location info for the files that will contribute to this config.
// All files will be used, and each will be merged on top of the previous ones. The first
// file must exist (in at least one of the search paths), but subsequent files are
// optional. The intention is that the first file is the primary config, and the other
// files optionally override that.
//
// The returned readers and readerNames are intended to be passed directly to configloader.Load().
// The closers should be closed after Load() is called, perhaps like this:
//  defer func() {
//    for i := range closers {
//      closers[i].Close()
//    }
//  }()
// The reason the closers are separate from the readers (instead of []io.ReadClosers) is
// both to ease passing into Load() and to help ensure the closing happens (via and
// "unused variable" compile error).
func FindFiles(fileLocations ...FileLocation) (readers []io.Reader, closers []io.Closer, readerNames []string, err error) {
	if len(fileLocations) == 0 {
		err = errors.Errorf("no filenames provided")
		return nil, nil, nil, err
	}

	defer func() {
		// In case of error, close the closers
		if err != nil {
			for i := range closers {
				closers[i].Close()
			}
		}
	}()

FilenamesLoop:
	for i, loc := range fileLocations {
		for _, path := range loc.SearchPaths {
			fpath := filepath.Join(path, loc.Filename)
			var f *os.File
			f, err := osOpen(fpath)
			if os.IsNotExist(err) {
				continue
			} else if err != nil {
				err = errors.Wrapf(err, "file open failed for %s", fpath)
				return nil, nil, nil, err
			}

			readers = append(readers, f)
			closers = append(closers, f)
			readerNames = append(readerNames, filepath.ToSlash(fpath))
			continue FilenamesLoop
		}

		// We failed to find the file in the search paths. This is only an error if this
		// is the first filename in filenames (i.e., not an override).
		if i == 0 {
			err = errors.Errorf("failed to find file '%v' in search paths: %+v", loc.Filename, loc.SearchPaths)
			return nil, nil, nil, err
		}
	}

	return readers, closers, readerNames, nil
}
