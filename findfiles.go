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

// Assigning to a variable to assist with testing (can force errors)
var osOpen = os.Open

// TODO: Comment
// filenames are the names of the files that will contribute to this config. All files will
// be used, and each will be merged on top of the previous ones. The first filename must
// exist, but subsequent names are optional. The intention is that the first file is the
// primary config, and the other files optionally override that.
// searchPaths is the set of paths where the config files will be looked for, in order.
// When the file is found, the search will stop. If this is set to {""}, the filenames
// will be used unmodified. (So, absolute paths could be set in filenames and they will be
// used directly.)
func FindConfigFiles(filenames, searchPaths []string) (readers []io.Reader, closers []io.Closer, readerNames []string, err error) {
	if len(filenames) == 0 {
		err = errors.Errorf("no filenames provided")
		return nil, nil, nil, err
	}

	if len(searchPaths) == 0 {
		err = errors.Errorf("no searchPaths provided")
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
	for i, fname := range filenames {
		for _, path := range searchPaths {
			fpath := filepath.Join(path, fname)
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
			err = errors.Errorf("failed to find file '%v' in search paths: %+v", fname, searchPaths)
			return nil, nil, nil, err
		}
	}

	return readers, closers, readerNames, nil
}
