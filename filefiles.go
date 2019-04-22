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
// filenames is the names of the files that will contribute to this config. All files will
// be used, and each will be merged on top of the previous ones. The first filename must
// exist, but subsequent names are optional. The intention is that the first file is the
// primary config, and the other files optionally override that.
// searchPaths is the set of paths where the config files will be looked for, in order.
// When the file is found, the search will stop. If this is set to {""}, the filenames
// will be used unmodified. (So, absolute paths could be set in filenames and they will be
// used directly.)
func FindConfigFiles(filenames, searchPaths []string) (readClosers []io.ReadCloser, readerNames []string, err error) {
	if len(filenames) == 0 {
		err = errors.Errorf("no filenames provided")
		return nil, nil, err
	}

	if len(searchPaths) == 0 {
		err = errors.Errorf("no searchPaths provided")
		return nil, nil, err
	}

	readClosers = make([]io.ReadCloser, len(filenames))
	readerNames = make([]string, len(filenames))

FilenamesLoop:
	for i, fname := range filenames {
		for _, path := range searchPaths {
			fpath := filepath.Join(path, fname)
			var f *os.File
			f, err := osOpen(fpath)
			if os.IsNotExist(err) {
				continue
			} else if err != nil {
				for _, rc := range readClosers {
					if rc != nil {
						rc.Close()
					}
				}

				err = errors.Wrapf(err, "file open failed for %s", fpath)
				return nil, nil, err
			}

			readClosers[i] = f
			readerNames[i] = filepath.ToSlash(fpath)
			continue FilenamesLoop
		}

		// We failed to find the file in the search paths
		err = errors.Errorf("failed to find file '%v' in search paths: %+v", fname, searchPaths)
		return nil, nil, err
	}

	return readClosers, readerNames, nil
}
