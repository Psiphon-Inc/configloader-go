package configloader

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestFindConfigFiles(t *testing.T) {
	type args struct {
		filenames   []string
		searchPaths []string
	}
	tests := []struct {
		name            string
		args            args
		osOpen          func(name string) (*os.File, error)
		wantReaderNames []string
		wantErr         bool
	}{
		{
			name: "error: no such file",
			args: args{
				filenames:   []string{"nonexistent"},
				searchPaths: []string{"testdata"},
			},
			wantErr: true,
		},
		{
			name: "error: no such directory",
			args: args{
				filenames:   []string{"file1"},
				searchPaths: []string{"nonexistent"},
			},
			wantErr: true,
		},
		{
			name: "simple; one file, one path",
			args: args{
				filenames:   []string{"file1"},
				searchPaths: []string{"testdata"},
			},
			wantReaderNames: []string{
				"testdata/file1",
			},
			wantErr: false,
		},
		{
			name: "one file, alternate path (hits first, nonexistent second dir)",
			args: args{
				filenames:   []string{"file1"},
				searchPaths: []string{"testdata", "nonexistent"},
			},
			wantReaderNames: []string{
				"testdata/file1",
			},
			wantErr: false,
		},
		{
			name: "one file, alternate path (hits first, existent second dir but no file)",
			args: args{
				filenames:   []string{"file2"},
				searchPaths: []string{"testdata", "testdata/subdir1"},
			},
			wantReaderNames: []string{
				"testdata/file2",
			},
			wantErr: false,
		},
		{
			name: "one file, alternate path (hits second, nonexistent first dir)",
			args: args{
				filenames:   []string{"file1"},
				searchPaths: []string{"nonexistent", "testdata"},
			},
			wantReaderNames: []string{
				"testdata/file1",
			},
			wantErr: false,
		},
		{
			name: "one file, alternate path (hits second, existent first dir but no file)",
			args: args{
				filenames:   []string{"file3"},
				searchPaths: []string{"testdata", "testdata/subdir1"},
			},
			wantReaderNames: []string{
				"testdata/subdir1/file3",
			},
			wantErr: false,
		},
		{
			name: "mutiple files",
			args: args{
				filenames:   []string{"file1", "file2", "file3"},
				searchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
			},
			wantReaderNames: []string{
				"testdata/file1",
				"testdata/file2",
				"testdata/subdir1/file3",
			},
			wantErr: false,
		},
		{
			name: "error: file open error",
			args: args{
				filenames:   []string{"file1", "file2", "file3"},
				searchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
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
			name: "error: no filenames provided",
			args: args{
				filenames:   []string{},
				searchPaths: []string{"testdata", "testdata/nonexistent", "testdata/subdir1"},
			},
			wantErr: true,
		},
		{
			name: "error: no search paths provided",
			args: args{
				filenames:   []string{"file1", "file2", "file3"},
				searchPaths: []string{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.osOpen != nil {
				osOpen = tt.osOpen
			}

			gotReadClosers, gotReaderNames, err := FindConfigFiles(tt.args.filenames, tt.args.searchPaths)

			// Restore the original function
			osOpen = os.Open

			if (err != nil) != tt.wantErr {
				t.Fatalf("FindConfigFiles() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(gotReaderNames, tt.wantReaderNames) {
				t.Fatalf("FindConfigFiles() gotReaderNames = %v, want %v", gotReaderNames, tt.wantReaderNames)
			}

			if len(gotReadClosers) != len(gotReaderNames) {
				t.Fatalf("length mismatch: len(gotReadClosers)=%v; len(gotReaderNames)=%v", len(gotReadClosers), len(gotReaderNames))
			}

			// We can't compare readClosers, but our test files simply contain the
			// path+filename, so we can read and check.
			for i := range gotReadClosers {
				buf, err := ioutil.ReadAll(gotReadClosers[i])
				gotReadClosers[i].Close()
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
