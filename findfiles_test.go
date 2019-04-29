/*
 * BSD 3-Clause License
 * Copyright (c) 2019, Psiphon Inc.
 * All rights reserved.
 */

package configloader

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestFindFiles(t *testing.T) {
	tests := []struct {
		name            string
		fileLocations   []FileLocation
		osOpen          func(name string) (*os.File, error)
		wantReaderNames []string
		wantErr         bool
	}{
		{
			name: "error: no such file",
			fileLocations: []FileLocation{
				{
					Filename:    "nonexistent",
					SearchPaths: []string{"testdata"},
				},
			},
			wantErr: true,
		},
		{
			name: "error: no such directory",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{"nonexistent"},
				},
			},
			wantErr: true,
		},
		{
			name: "simple; one file, one path",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{"testdata"},
				},
			},
			wantReaderNames: []string{
				"testdata/file1",
			},
			wantErr: false,
		},
		{
			name: "empty path, path in filename",
			fileLocations: []FileLocation{
				{
					Filename:    "testdata/file1",
					SearchPaths: []string{""},
				},
			},
			wantReaderNames: []string{
				"testdata/file1",
			},
			wantErr: false,
		},
		{
			name: "one file, alternate path (hits first, nonexistent second dir)",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{"testdata", "nonexistent"},
				},
			},
			wantReaderNames: []string{
				"testdata/file1",
			},
			wantErr: false,
		},
		{
			name: "one file, alternate path (hits first, existent second dir but no file)",
			fileLocations: []FileLocation{
				{
					Filename:    "file2",
					SearchPaths: []string{"testdata", "testdata/subdir1"},
				},
			},
			wantReaderNames: []string{
				"testdata/file2",
			},
			wantErr: false,
		},
		{
			name: "one file, alternate path (hits second, nonexistent first dir)",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{"nonexistent", "testdata"},
				},
			},
			wantReaderNames: []string{
				"testdata/file1",
			},
			wantErr: false,
		},
		{
			name: "one file, alternate path (hits second, existent first dir but no file)",
			fileLocations: []FileLocation{
				{
					Filename:    "file3",
					SearchPaths: []string{"testdata", "testdata/subdir1"},
				},
			},
			wantReaderNames: []string{
				"testdata/subdir1/file3",
			},
			wantErr: false,
		},
		{
			name: "mutiple files, all existing and all paths existing",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{"testdata", "testdata/subdir1"},
				},
				{
					Filename:    "file2",
					SearchPaths: []string{"testdata", "testdata/subdir1"},
				},
				{
					Filename:    "file3",
					SearchPaths: []string{"testdata", "testdata/subdir1"},
				},
			},
			wantReaderNames: []string{
				"testdata/file1",
				"testdata/file2",
				"testdata/subdir1/file3",
			},
			wantErr: false,
		},
		{
			name: "mutiple files, all existing, some paths not existing",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
				{
					Filename:    "file2",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
				{
					Filename:    "file3",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
			},
			wantReaderNames: []string{
				"testdata/file1",
				"testdata/file2",
				"testdata/subdir1/file3",
			},
			wantErr: false,
		},
		{
			name: "mutiple files, no overrides existing",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{"testdata/subdir1", "testdata", "testdata/nonexistent"},
				},
				{
					Filename:    "file1_override1",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
				{
					Filename:    "file1_override2",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
			},
			wantReaderNames: []string{
				"testdata/subdir1/file1",
			},
			wantErr: false,
		},
		{
			name: "mutiple files, some overrides existing",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
				{
					Filename:    "file1_override1",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
				{
					Filename:    "file3",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
			},
			wantReaderNames: []string{
				"testdata/file1",
				"testdata/subdir1/file3",
			},
			wantErr: false,
		},
		{
			name: "error: file open error",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
				{
					Filename:    "file2",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
				{
					Filename:    "file3",
					SearchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
				},
			},
			osOpen: func(name string) (*os.File, error) {
				if strings.HasSuffix(name, "file2") {
					return nil, fmt.Errorf("oh no file open failed")
				}
				return os.Open(name)
			},
			wantErr: true,
		},
		{
			name:          "error: no file locations provided",
			fileLocations: []FileLocation{},
			wantErr:       true,
		},
		{
			name: "error: no search paths provided",
			fileLocations: []FileLocation{
				{
					Filename:    "file1",
					SearchPaths: []string{},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.osOpen != nil {
				osOpen = tt.osOpen
			}

			gotReaders, gotClosers, gotReaderNames, err := FindFiles(tt.fileLocations...)

			defer func() {
				for i := range gotClosers {
					if err := gotClosers[i].Close(); err != nil {
						t.Fatalf("failed to close closer for '%v': %v", gotReaderNames[i], err)
					}
				}
			}()

			// Restore the original function
			osOpen = os.Open

			if (err != nil) != tt.wantErr {
				t.Fatalf("FindFiles() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(gotReaderNames, tt.wantReaderNames) {
				t.Fatalf("FindFiles() gotReaderNames = %v, want %v", gotReaderNames, tt.wantReaderNames)
			}

			if len(gotReaders) != len(gotReaderNames) {
				t.Fatalf("length mismatch: len(gotReaders)=%v; len(gotReaderNames)=%v", len(gotReaders), len(gotReaderNames))
			}

			if len(gotReaders) != len(gotClosers) {
				t.Fatalf("length mismatch: len(gotReaders)=%v; len(gotClosers)=%v", len(gotReaders), len(gotClosers))
			}

			// We can't compare io.Readers, but our test files simply contain the
			// path+filename, so we can read and check.
			for i := range gotReaders {
				buf, err := ioutil.ReadAll(gotReaders[i])
				if err != nil {
					t.Fatalf("failed to read readCloser with name '%v'", gotReaderNames[i])
				}

				fileContents := strings.TrimSpace(string(buf))
				if fileContents != gotReaderNames[i] {
					t.Fatalf("file contents should match reader name;\nfileContents: %v\nreaderName: %v", fileContents, gotReaderNames[i])
				}
			}
		})
	}
}
