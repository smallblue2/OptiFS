package metadata

import (
	"bytes"
	"reflect"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"lukechampine.com/blake3"
)

// Helper function for comparing MapEntryMetadata structs
func compareMetadata(t *testing.T, expected, actual *MapEntryMetadata) {
	if actual.Path != expected.Path {
		t.Errorf("Path: expected {%v} - received {%v}\n", expected.Path, actual.Path)
	}
	if actual.Ino != expected.Ino {
		t.Errorf("Ino: expected {%v} - received {%v}\n", expected.Ino, actual.Ino)
	}
	if actual.Gen != expected.Gen {
		t.Errorf("Gen: expected {%v} - received {%v}\n", expected.Gen, actual.Gen)
	}
	if actual.Mode != expected.Mode {
		t.Errorf("Mode: expected {%v} - received {%v}\n", expected.Mode, actual.Mode)
	}
	if actual.Dev != expected.Dev {
		t.Errorf("Dev: expected {%v} - received {%v}\n", expected.Dev, actual.Dev)
	}
	if actual.Nlink != expected.Nlink {
		t.Errorf("Nlink: expected {%v} - received {%v}\n", expected.Nlink, actual.Nlink)
	}
	if actual.Uid != expected.Uid {
		t.Errorf("Uid: expected {%v} - received {%v}\n", expected.Uid, actual.Uid)
	}
	if actual.Gid != expected.Gid {
		t.Errorf("Gid: expected {%v} - received {%v}\n", expected.Gid, actual.Gid)
	}
	if actual.Size != expected.Size {
		t.Errorf("Size: expected {%v} - received {%v}\n", expected.Size, actual.Size)
	}
	if actual.Blocks != expected.Blocks {
		t.Errorf("Blocks: expected {%v} - received {%v}\n", expected.Blocks, actual.Blocks)
	}
	if actual.Blksize != expected.Blksize {
		t.Errorf("Blksize: expected {%v} - received {%v}\n", expected.Blksize, actual.Blksize)
	}
	if actual.Atim != expected.Atim {
		t.Errorf("Atim: expected {%v} - received {%v}\n", expected.Atim, actual.Atim)
	}
	if actual.Ctim != expected.Ctim {
		t.Errorf("Ctim: expected {%v} - received {%v}\n", expected.Ctim, actual.Ctim)
	}
	if actual.Mtim != expected.Mtim {
		t.Errorf("Mtim: expected {%v} - received {%v}\n", expected.Mtim, actual.Mtim)
	}
	if actual.Rdev != expected.Rdev {
		t.Errorf("Rdev: expected {%v} - received {%v}\n", expected.Rdev, actual.Rdev)
	}
	if actual.X__pad0 != expected.X__pad0 {
		t.Errorf("X__pad0: expected {%v} - received {%v}\n", expected.X__pad0, actual.X__pad0)
	}
	if actual.X__unused != expected.X__unused {
		t.Errorf("X__unused: expected {%v} - received {%v}\n", expected.X__unused, actual.X__unused)
	}
	if actual.X__unused != expected.X__unused {
		t.Errorf("X__unused: expected {%v} - received {%v}\n", expected.X__unused, actual.X__unused)
	}
	if actual.XAttr == nil {
		t.Error("XAttr map wasn't initialised!")
	}
}

func generateExpectedMetadata(unstableAttr *syscall.Stat_t, stableAttr *fs.StableAttr, tmpFilePath string) *MapEntryMetadata {
	expected := &MapEntryMetadata{Path: tmpFilePath, Ino: stableAttr.Ino, Gen: stableAttr.Gen, Mode: unstableAttr.Mode, Dev: unstableAttr.Dev, Nlink: unstableAttr.Nlink, Uid: unstableAttr.Uid, Gid: unstableAttr.Gid, Size: unstableAttr.Size, Blocks: unstableAttr.Blocks, Blksize: unstableAttr.Blksize, Atim: unstableAttr.Atim, Ctim: unstableAttr.Ctim, Mtim: unstableAttr.Mtim, Rdev: unstableAttr.Rdev, X__unused: unstableAttr.X__unused, X__pad0: unstableAttr.X__pad0, XAttr: make(map[string][]byte)}
	return expected
}

// Unit test for UpdateAllFromStat in common.go
func TestUpdateAllFromStat(t *testing.T) {

	tmpDir := t.TempDir()

	testCases := []struct {
		FilePath     string
		UnstableAttr *syscall.Stat_t
		StableAttr   *fs.StableAttr
	}{
		{
			FilePath:     tmpDir + "/file1",
			UnstableAttr: &syscall.Stat_t{},
			StableAttr:   &fs.StableAttr{Ino: 123456, Gen: 0, Mode: 0},
		},
		{
			FilePath:     tmpDir + "/file2",
			UnstableAttr: &syscall.Stat_t{},
			StableAttr:   &fs.StableAttr{Ino: 12, Gen: 3, Mode: 43221},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.FilePath, func(t *testing.T) {
			// Prepare file descriptor 'fd' ...
			fd, err := syscall.Creat(testCase.FilePath, 666)
			if err != nil {
				t.Fatalf("Failed to create file - %v\n", err)
			}
			syscall.Fstat(fd, testCase.UnstableAttr)
			stubMetadata := &MapEntryMetadata{}
			updateAllFromStat(stubMetadata, testCase.UnstableAttr, testCase.StableAttr, testCase.FilePath)
			expected := generateExpectedMetadata(testCase.UnstableAttr, testCase.StableAttr, testCase.FilePath)
			compareMetadata(t, expected, stubMetadata)
		})
	}
}

// Unit test for GetCustomXAttr in common.go
func TestGetCustomXAttr(t *testing.T) {
	testCases := []struct {
		name        string
		metadata    *MapEntryMetadata
		attr        string
		isDir       bool
		expectedLen uint32
		expectedErr syscall.Errno
	}{
		{
			name:        "xattr missing",
			metadata:    &MapEntryMetadata{}, // No XAttr map
			attr:        "testAttr",
			isDir:       false,
			expectedLen: 0,
			expectedErr: fs.ToErrno(syscall.ENODATA),
		},
		{
			name: "xattr exists",
			metadata: &MapEntryMetadata{
				XAttr: map[string][]byte{
					"testAttr": []byte("hello"),
					"other":    []byte("world"),
				},
			},
			attr:        "testAttr",
			isDir:       true,
			expectedLen: 5,
			expectedErr: fs.OK,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			dest := make([]byte, 10) // Provide a suitable buffer size
			size, err := GetCustomXAttr(tt.metadata, tt.attr, &dest, tt.isDir)

			if size != tt.expectedLen {
				t.Errorf("Incorrect size. Expected: %d, Got: %d", tt.expectedLen, size)
			}
			if err != tt.expectedErr {
				t.Errorf("Incorrect error. Expected: %v, Got: %v", tt.expectedErr, err)
			}
			if tt.expectedErr == fs.OK {
				if !bytes.Equal(dest[:size], []byte(tt.metadata.XAttr[tt.attr])) {
					t.Errorf("Incorrect xattr value in dest buffer")
				}
			}
		})
	}
}

// Unit tests for SetCustomXAttr in common.go
func TestSetCustomXAttr(t *testing.T) {
	testCases := []struct {
		name        string
		metadata    *MapEntryMetadata
		attr        string
		data        []byte
		flags       uint32
		expectedErr syscall.Errno
	}{
		{
			name: "create new xattr",
			metadata: &MapEntryMetadata{
				XAttr: map[string][]byte{}, // Or nil
			},
			attr:        "newAttr",
			data:        []byte("testValue"),
			flags:       0x1, // XATTR_CREATE
			expectedErr: fs.OK,
		},
		{
			name: "XATTR_CREATE failure (attr exists)",
			metadata: &MapEntryMetadata{
				XAttr: map[string][]byte{
					"testAttr": []byte("existing"),
				},
			},
			attr:        "testAttr",
			data:        []byte("newValue"),
			flags:       0x1, // XATTR_CREATE
			expectedErr: fs.ToErrno(syscall.EEXIST),
		},
		{
			name: "XATTR_REPLACE success",
			metadata: &MapEntryMetadata{
				XAttr: map[string][]byte{
					"replaceAttr": []byte("old"),
				},
			},
			attr:        "replaceAttr",
			data:        []byte("updated"),
			flags:       0x2, // XATTR_REPLACE
			expectedErr: fs.OK,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			err := SetCustomXAttr(tt.metadata, tt.attr, tt.data, tt.flags, false)

			if err != tt.expectedErr {
				t.Errorf("Incorrect error. Expected: %v, Got: %v", tt.expectedErr, err)
			}
			if tt.expectedErr == fs.OK {
				// Additional check on the XAttr map to verify expected  changes
				if !bytes.Equal(tt.metadata.XAttr[tt.attr], tt.data) {
					t.Errorf("XAttr not set/updated correctly")
				}
			}
		})
	}
}

// Unit test for RemoveCustomXAttr in common.go
func TestRemoveCustomXAttr(t *testing.T) {
	testCases := []struct {
		name        string
		metadata    *MapEntryMetadata
		attr        string
		isDir       bool
		expectedErr syscall.Errno
	}{
		{
			name: "xattr removal success",
			metadata: &MapEntryMetadata{
				XAttr: map[string][]byte{
					"removeAttr": []byte("data"),
					"otherAttr":  []byte("keepThis"),
				},
			},
			attr:        "removeAttr",
			isDir:       true,
			expectedErr: fs.OK,
		},
		{
			name: "remove nonexistent xattr",
			metadata: &MapEntryMetadata{
				XAttr: map[string][]byte{},
			},
			attr:        "missingAttr",
			isDir:       false,
			expectedErr: fs.ToErrno(syscall.ENODATA),
		},
		{
			name:        "error - nil metadata",
			metadata:    nil,
			attr:        "anything",
			isDir:       false,
			expectedErr: fs.ToErrno(syscall.ENODATA),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			err := RemoveCustomXAttr(tt.metadata, tt.attr, tt.isDir)

			if err != tt.expectedErr {
				t.Errorf("Incorrect error. Expected: %v, Got: %v", tt.expectedErr, err)
			}

			if tt.expectedErr == fs.OK {
				_, exists := tt.metadata.XAttr[tt.attr]
				if exists {
					t.Error("xattr was not removed successfully")
				}
			}
		})
	}
}

// Unit test for ListCustomXAttr
func TestListCustomXAttr(t *testing.T) {
	testCases := []struct {
		name         string
		metadata     *MapEntryMetadata
		bufferSize   int // Size of the provided `dest` buffer
		expectedErr  syscall.Errno
		expectedLen  uint32
		expectedDest string
	}{
		{
			name:         "nil metadata",
			metadata:     nil,
			bufferSize:   100,
			expectedErr:  fs.ToErrno(syscall.ENODATA),
			expectedLen:  0,
			expectedDest: "",
		},
		{
			name:         "empty xattrs",
			metadata:     &MapEntryMetadata{XAttr: map[string][]byte{}},
			bufferSize:   50,
			expectedErr:  fs.OK,
			expectedLen:  0,
			expectedDest: "",
		},
		{
			name: "buffer too small",
			metadata: &MapEntryMetadata{
				XAttr: map[string][]byte{
					"attr1":             nil,
					"longAttributeName": nil,
				},
			},
			bufferSize:   5,
			expectedErr:  fs.ToErrno(syscall.ERANGE),
			expectedLen:  24, // Total length needed including null terminators
			expectedDest: "",
		},
		{
			name: "success - listing",
			metadata: &MapEntryMetadata{
				XAttr: map[string][]byte{
					"xattr1": nil,
					"test":   nil,
					"foo":    nil,
				},
			},
			bufferSize:   20,
			expectedErr:  fs.OK,
			expectedLen:  16,
			expectedDest: "foo\x00test\x00xattr1\x00",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			dest := make([]byte, tt.bufferSize)
			size, err := ListCustomXAttr(tt.metadata, &dest, false) // isDir as needed

			if size != tt.expectedLen {
				t.Errorf("Incorrect size. Expected: %d, Got: %d", tt.expectedLen, size)
			}
			if err != tt.expectedErr {
				t.Errorf("Incorrect error. Expected: %v, Got: %v", tt.expectedErr, err)
			}
			if tt.expectedErr == fs.OK {
				if string(dest[:size]) != tt.expectedDest {
					t.Errorf("Incorrect listing in dest. Expected: %q, Got: %q", tt.expectedDest, dest[:size])
				}
			}
		})
	}
}

func blake3Hash(in []byte) (out [64]byte) {
	return blake3.Sum512(in)
}

// Unit test for IsContentHashUnique function in regular_file_metadata_api.go
func TestIsContentHashUnique(t *testing.T) {
	testCases := []struct {
		name                string
		inputHash           [64]byte
		mockHashMap         map[[64]byte]*MapEntry // Simulated dependency
		expectedReturnValue bool
	}{
		{
			name:                "Empty hash",
			inputHash:           [64]byte{}, // An empty/default hash
			mockHashMap:         map[[64]byte]*MapEntry{},
			expectedReturnValue: true,
		},
		{
			name:      "Non-existent hash",
			inputHash: blake3Hash([]byte{1, 2, 3, 4}), // Randomly generated hash
			mockHashMap: map[[64]byte]*MapEntry{
				blake3Hash([]byte{5, 6, 7, 8}): nil}, // Non-empty map
			expectedReturnValue: true,
		},
		{
			name:      "Existing hash",
			inputHash: blake3Hash([]byte{5, 6, 7, 8}), // Randomly generated hash
			mockHashMap: map[[64]byte]*MapEntry{ // Non-empty map
				blake3Hash([]byte{5, 6, 7, 8}):             nil,
				blake3Hash([]byte{1, 2, 3, 4, 5, 6, 7, 8}): nil,
				blake3Hash([]byte{9, 8, 7, 6, 5, 4}):       nil,
			},
			expectedReturnValue: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Replace regularFileMetadataHash with the mock version during the test
			regularFileMetadataHash = tc.mockHashMap

			result := IsContentHashUnique(tc.inputHash)

			if result != tc.expectedReturnValue {
				t.Errorf("Expected %v, got %v", tc.expectedReturnValue, result)
			}
		})
	}
}

// Unit test for IsContentHashUnique function in regular_file_metadata_api.go
func TestCreateDirEntry(t *testing.T) {
	testCases := []struct {
		name                string
		inputPath           string
		mockHashMap         map[string]*MapEntryMetadata // Simulated dependency
		expectedReturnValue *MapEntryMetadata
	}{
		{
			name:                "Normal input",
			inputPath:           "test/directory",
			mockHashMap:         map[string]*MapEntryMetadata{},
			expectedReturnValue: &MapEntryMetadata{},
		},
		{
			name:      "Existing input",
			inputPath: "test/directory",
			mockHashMap: map[string]*MapEntryMetadata{
				"test/directory": {Ino: 12}},
			expectedReturnValue: &MapEntryMetadata{XAttr: map[string][]byte{}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Replace regularFileMetadataHash with the mock version during the test
			dirMetadataHash = tc.mockHashMap

			result := CreateDirEntry(tc.inputPath)

			compareMetadata(t, &MapEntryMetadata{}, result)
			compareMetadata(t, &MapEntryMetadata{}, dirMetadataHash[tc.inputPath])
		})
	}
}

// Unit test for LookupDirMetadata in directory_metadata_api.go
func TestLookupDirMetadata(t *testing.T) {
	testCases := []struct {
		name                string
		inputPath           string
		mockHashMap         map[string]*MapEntryMetadata // Simulated dependency
		expectedReturnValue *MapEntryMetadata
		expectedReturnError syscall.Errno
	}{
		{
			name:      "Existing lookup",
			inputPath: "test/directory",
			mockHashMap: map[string]*MapEntryMetadata{
				"test/directory":     {},
				"other/random/place": {}},
			expectedReturnValue: &MapEntryMetadata{},
			expectedReturnError: 0,
		},
		{
			name:      "Non-existing lookup",
			inputPath: "test/directory",
			mockHashMap: map[string]*MapEntryMetadata{
				"random/other": {}},
			expectedReturnValue: nil,
			expectedReturnError: fs.ToErrno(syscall.ENODATA),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Replace regularFileMetadataHash with the mock version during the test
			dirMetadataHash = tc.mockHashMap

			err, result := LookupDirMetadata(tc.inputPath)

			if err != tc.expectedReturnError {
				t.Errorf("Expected %v, got %v\n", tc.expectedReturnError, err)
			}
			if err == 0 {
				if result == nil {
					t.Errorf("Expected %v, got %v\n", tc.expectedReturnValue, result)
				}
			}
		})
	}
}

// Unit test for UpdateDirEntry in directory_metadata_api.go
func TestUpdateDirEntry(t *testing.T) {
	testCases := []struct {
		name                string
		inputPath           string
		mockHashMap         map[string]*MapEntryMetadata // Simulated dependency
		mockStable          *fs.StableAttr
		mockUnstable        *syscall.Stat_t
		expectedReturnValue *MapEntryMetadata
		expectedReturnError syscall.Errno
	}{
		// Existing entry and succesful update
		{
			name:      "Succesful update",
			inputPath: "foo/bar",
			mockHashMap: map[string]*MapEntryMetadata{
				"foo/bar":                    {},
				"another/one":                {},
				"hello":                      {},
				"this/is/a/longer/directory": {},
			},
			mockStable:          &fs.StableAttr{Gen: 12, Ino: 12345, Mode: 5323},
			mockUnstable:        &syscall.Stat_t{Ino: 54321, Dev: 3, Mode: 4321, Uid: 1000, Gid: 1000, Rdev: 21, Size: 3454322, Atim: syscall.NsecToTimespec(1232), Mtim: syscall.NsecToTimespec(4322), Ctim: syscall.NsecToTimespec(1), Nlink: 2, Blocks: 12, Blksize: 5432, X__pad0: 12, X__unused: [3]int64{1, 2, 3}},
			expectedReturnError: 0,
		},
		// Entry doesn't exist
		{
			name:      "Entry doesn't exist",
			inputPath: "hello/world",
			mockHashMap: map[string]*MapEntryMetadata{
				"foo/bar":                    {},
				"another/one":                {},
				"hello":                      {},
				"this/is/a/longer/directory": {},
			},
			mockStable:          &fs.StableAttr{},
			mockUnstable:        &syscall.Stat_t{},
			expectedReturnValue: &MapEntryMetadata{},
			expectedReturnError: fs.ToErrno(syscall.ENODATA),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Replace regularFileMetadataHash with the mock version during the test
			dirMetadataHash = tc.mockHashMap

			err := UpdateDirEntry(tc.inputPath, tc.mockUnstable, tc.mockStable)

			if err != tc.expectedReturnError {
				t.Errorf("Expected %v, got %v\n", tc.expectedReturnError, err)
			}
			if err == 0 {
				expected := generateExpectedMetadata(tc.mockUnstable, tc.mockStable, tc.inputPath)
				compareMetadata(t, expected, dirMetadataHash[tc.inputPath])
			}
		})
	}
}

// Unit test for RemoveDirEntry in directory_metadata_api.go
func TestRemoveDirEntry(t *testing.T) {
	testCases := []struct {
		name          string
		inputPath     string
		mockHashMap   map[string]*MapEntryMetadata
		expectedState map[string]*MapEntryMetadata // State of hashmap after RemoveDirEntry
	}{
		{
			name:          "Entry Exists",
			inputPath:     "test/path",
			mockHashMap:   map[string]*MapEntryMetadata{"test/path": {}},
			expectedState: map[string]*MapEntryMetadata{}, // Entry should be removed
		},
		{
			name:          "Entry Does Not Exist",
			inputPath:     "test/missing",
			mockHashMap:   map[string]*MapEntryMetadata{"other/path": {}},
			expectedState: map[string]*MapEntryMetadata{"other/path": {}}, // No change expected
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dirMetadataHash = tc.mockHashMap

			RemoveDirEntry(tc.inputPath)

			// Assertion: Using reflect.DeepEqual for map comparison
			if !reflect.DeepEqual(dirMetadataHash, tc.expectedState) {
				t.Errorf("Hashmap state mismatch!\nExpected: %v\nGot: %v", tc.expectedState, dirMetadataHash)
			}
		})
	}
}

// Unit test for FillAttr in general_api.go
func TestFillAttr(t *testing.T) {
	testCases := []struct {
		name          string
		inputMetadata *MapEntryMetadata
		expectedAttr  fuse.Attr
	}{
		{
			name: "Typical Data Transfer",
			inputMetadata: &MapEntryMetadata{
				Ino:     12345,
				Size:    4096,
				Blocks:  8,
				Atim:    syscall.Timespec{Sec: 1679009770, Nsec: 62539149},
				Mtim:    syscall.Timespec{Sec: 1679009775, Nsec: 869650321},
				Ctim:    syscall.Timespec{Sec: 1679009772, Nsec: 797906618},
				Mode:    0755,
				Nlink:   2,
				Uid:     1000,
				Gid:     1000,
				Rdev:    0,
				Blksize: 512,
				// Add any other relevant fields
			},
			expectedAttr: fuse.Attr{
				Ino:       12345,
				Size:      4096,
				Blocks:    8,
				Atime:     1679009770,
				Atimensec: 62539149,
				Mtime:     1679009775,
				Mtimensec: 869650321,
				Ctime:     1679009772,
				Ctimensec: 797906618,
				Mode:      0755,
				Nlink:     2,
				Owner:     fuse.Owner{Uid: 1000, Gid: 1000},
				Rdev:      0,
				Blksize:   512,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualAttr := fuse.Attr{}

			FillAttr(tc.inputMetadata, &actualAttr)

			if !reflect.DeepEqual(actualAttr, tc.expectedAttr) {
				t.Errorf("Incorrect attribute filling. Expected: %+v, Got: %+v", tc.expectedAttr, actualAttr)
			}
		})
	}
}

// Unit test for FillAttrOut in general_api.go
func TestFillAttrOut(t *testing.T) {
	testCases := []struct {
		name          string
		inputMetadata *MapEntryMetadata
		expectedAttr  fuse.AttrOut
	}{
		{
			name: "Typical Data Transfer",
			inputMetadata: &MapEntryMetadata{
				Ino:     12345,
				Size:    4096,
				Blocks:  8,
				Atim:    syscall.Timespec{Sec: 1679009770, Nsec: 62539149},
				Mtim:    syscall.Timespec{Sec: 1679009775, Nsec: 869650321},
				Ctim:    syscall.Timespec{Sec: 1679009772, Nsec: 797906618},
				Mode:    0755,
				Nlink:   2,
				Uid:     1000,
				Gid:     1000,
				Rdev:    0,
				Blksize: 512,
				// Add any other relevant fields
			},
			expectedAttr: fuse.AttrOut{
				Attr: fuse.Attr{
					Ino:       12345,
					Size:      4096,
					Blocks:    8,
					Atime:     1679009770,
					Atimensec: 62539149,
					Mtime:     1679009775,
					Mtimensec: 869650321,
					Ctime:     1679009772,
					Ctimensec: 797906618,
					Mode:      0755,
					Nlink:     2,
					Owner:     fuse.Owner{Uid: 1000, Gid: 1000},
					Rdev:      0,
					Blksize:   512,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualAttr := fuse.AttrOut{}

			FillAttrOut(tc.inputMetadata, &actualAttr)

			if !reflect.DeepEqual(actualAttr, tc.expectedAttr) {
				t.Errorf("Incorrect attribute filling. Expected: %+v, Got: %+v", tc.expectedAttr, actualAttr)
			}
		})
	}
}

// Unit tests for StoreRegFileInfo in persistence_api.go
func TestStoreRegFileInfo(t *testing.T) {
	testCases := []struct {
		name          string
		path          string
		stableAttr    *fs.StableAttr
		mode          uint32
		contentHash   [64]byte
		refNum        uint64
		expectedState map[string]*NodeInfo
	}{
		{
			name: "New Entry",
			path: "test/file1",
			stableAttr: &fs.StableAttr{
				Gen:  12345,
				Ino:  56789,
				Mode: 0644,
			},
			mode:        0644,
			contentHash: blake3Hash([]byte{1, 2, 3, 4}),
			refNum:      1,
			expectedState: map[string]*NodeInfo{
				"test/file1": {
					StableGen:   12345,
					StableIno:   56789,
					StableMode:  0644,
					Mode:        0644,
					IsDir:       false,
					ContentHash: blake3Hash([]byte{1, 2, 3, 4}),
					RefNum:      1,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Redeclare nodePersistenceHash dependency
			nodePersistenceHash = make(map[string]*NodeInfo)

			StoreRegFileInfo(tc.path, tc.stableAttr, tc.mode, tc.contentHash, tc.refNum)

			if !reflect.DeepEqual(nodePersistenceHash, tc.expectedState) {
				t.Errorf("Incorrect hashmap state!\nExpected: %+v\nGot: %+v", tc.expectedState, nodePersistenceHash)
			}
		})
	}
}

// Unit tests for StoreDirInfo in persistence_api.go
func TestStoreDirInfo(t *testing.T) {
	testCases := []struct {
		name          string
		path          string
		stableAttr    *fs.StableAttr
		mode          uint32
		expectedState map[string]*NodeInfo
	}{
		{
			name: "New Entry",
			path: "test/file1",
			stableAttr: &fs.StableAttr{
				Gen:  12345,
				Ino:  56789,
				Mode: 0644,
			},
			mode: 0644,
			expectedState: map[string]*NodeInfo{
				"test/file1": {
					StableGen:   12345,
					StableIno:   56789,
					StableMode:  0644,
					Mode:        0644,
					IsDir:       true,
					ContentHash: [64]byte{},
					RefNum:      0,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Redeclare nodePersistenceHash dependency
			nodePersistenceHash = make(map[string]*NodeInfo)

			StoreDirInfo(tc.path, tc.stableAttr, tc.mode)

			if !reflect.DeepEqual(nodePersistenceHash, tc.expectedState) {
				t.Errorf("Incorrect hashmap state!\nExpected: %+v\nGot: %+v", tc.expectedState, nodePersistenceHash)
			}
		})
	}
}

// Helper functions to create optional value pointers
func boolPtr(b bool) *bool         { return &b }
func modePtr(i uint32) *uint32     { return &i }
func hashPtr(h [64]byte) *[64]byte { return &h }
func refPtr(r uint64) *uint64      { return &r }

// Unit test for UpdateNodeInfo in persistence_api.go
func TestUpdateNodeInfo(t *testing.T) {
	testCases := []struct {
		name          string
		path          string
		initialData   *NodeInfo // Data initially in nodePersistenceHash
		isDir         *bool
		stableAttr    *fs.StableAttr
		mode          *uint32
		contentHash   *[64]byte
		refNum        *uint64
		expectedState *NodeInfo
	}{
		{
			name: "Update IsDir Flag",
			path: "test/file",
			initialData: &NodeInfo{
				IsDir: false,
				// ... other initial fields ...
			},
			isDir: boolPtr(true), // Helper - converts bool to a pointer
			expectedState: &NodeInfo{
				IsDir: true,
				// ... other initial fields should remain unchanged ...
			},
		},
		{
			name:        "Update Stable Attributes",
			path:        "test/dir",
			initialData: &NodeInfo{ /* Existing values */ },
			stableAttr:  &fs.StableAttr{Gen: 54321, Ino: 98765, Mode: 0666},
			expectedState: &NodeInfo{
				StableGen:  54321,
				StableIno:  98765,
				StableMode: 0666,
			},
		},
		{
			name:        "Update Mode",
			path:        "test/dir",
			initialData: &NodeInfo{ /* Existing values */ },
			mode:        modePtr(644),
			expectedState: &NodeInfo{
				Mode: 644,
			},
		},
		{
			name:        "Update Hash and Refnum",
			path:        "test/dir",
			initialData: &NodeInfo{ /* Existing values */ },
			contentHash: hashPtr(blake3Hash([]byte{1, 2, 3, 4})),
			refNum:      refPtr(5),
			expectedState: &NodeInfo{
				ContentHash: blake3Hash([]byte{1, 2, 3, 4}),
				RefNum:      5,
			},
		},
		{
			name:          "Path Not Found",
			path:          "test/missing",
			initialData:   nil, // Not required when the path is absent
			expectedState: nil, // Or an empty NodeInfo if your logic creates something
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			nodePersistenceHash = map[string]*NodeInfo{}
			if tc.initialData != nil {
				nodePersistenceHash[tc.path] = tc.initialData
			}

			// Call the function
			UpdateNodeInfo(tc.path, tc.isDir, tc.stableAttr, tc.mode, tc.contentHash, tc.refNum)

			// Assertion
			result, exists := nodePersistenceHash[tc.path]
			if !exists && tc.expectedState == nil {
				// "Not exists" path and we expected it, success
				return
			}

			if !exists || !reflect.DeepEqual(result, tc.expectedState) {
				t.Errorf("Incorrect update or missing entry!\nExpected: %+v\nGot: %+v", tc.expectedState, result)
			}
		})
	}
}

// Unit test for RetrieveNodeInfo in persistence.go
func TestRetrieveNodeInfo(t *testing.T) {
	testCases := []struct {
		name          string
		path          string
		mockMap       map[string]*NodeInfo
		expectedError syscall.Errno
		exStIno       uint64
		exStMode      uint32
		exStGen       uint64
		exMode        uint32
		exIsDir       bool
		exHash        [64]byte
		exRefNum      uint64
	}{
		{
			name: "Retrieve entry",
			path: "test/file1",
			mockMap: map[string]*NodeInfo{
				"test/file1": {
					StableIno:   123,
					StableGen:   1,
					StableMode:  0644,
					Mode:        123,
					IsDir:       false,
					ContentHash: blake3Hash([]byte{1, 2, 3, 4}),
					RefNum:      32,
				},
				"test/file2": {},
			},
			exStIno:       123,
			exStGen:       1,
			exStMode:      0644,
			exMode:        123,
			exIsDir:       false,
			exHash:        blake3Hash([]byte{1, 2, 3, 4}),
			exRefNum:      32,
			expectedError: fs.OK,
		},
		{
			name: "No entry exists",
			path: "test/file4",
			mockMap: map[string]*NodeInfo{
				"test/file1": {
					StableIno:   123,
					StableGen:   1,
					StableMode:  0644,
					Mode:        123,
					IsDir:       false,
					ContentHash: blake3Hash([]byte{1, 2, 3, 4}),
					RefNum:      32,
				},
				"test/file2": {},
			},
			exStIno:       0,
			exStGen:       0,
			exStMode:      0,
			exMode:        0,
			exIsDir:       false,
			exHash:        [64]byte{},
			exRefNum:      0,
			expectedError: fs.ToErrno(syscall.ENODATA),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Redeclare nodePersistenceHash dependency
			nodePersistenceHash = tc.mockMap

			err, stIno, stMode, stGen, Mode, isDir, hash, ref := RetrieveNodeInfo(tc.path)

			if err != tc.expectedError {
				t.Errorf("Expected %v, got %v\n", tc.expectedError, err)
			}
			if stIno != tc.exStIno {
				t.Errorf("Expected %v, got %v\n", tc.exStIno, stIno)
			}
			if stMode != tc.exStMode {
				t.Errorf("Expected %v, got %v\n", tc.exStMode, stMode)
			}
			if stGen != tc.exStGen {
				t.Errorf("Expected %v, got %v\n", tc.exStGen, stGen)
			}
			if Mode != tc.exMode {
				t.Errorf("Expected %v, got %v\n", tc.exMode, Mode)
			}
			if isDir != tc.exIsDir {
				t.Errorf("Expected %v, got %v\n", tc.exIsDir, isDir)
			}
			if hash != tc.exHash {
				t.Errorf("Expected %v, got %v\n", tc.exHash, hash)
			}
			if ref != tc.exRefNum {
				t.Errorf("Expected %v, got %v\n", tc.exRefNum, ref)
			}

		})
	}
}

// Unit test for EmptyFileIdentifier function in regular_file_metadata_api.go
func TestEmptyFileIdentifier(t *testing.T) {
	testcases := []struct {
		name           string
		contentHash    [64]byte
		expectedReturn bool
	}{
		{
			name:           "Default 64byte bool",
			contentHash:    [64]byte{},
			expectedReturn: true,
		},
		{
			name:           "Non-default 64byte bool",
			contentHash:    blake3Hash([]byte("hello")),
			expectedReturn: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			returnValue := EmptyFileIdentifier(tc.contentHash)
			if returnValue != tc.expectedReturn {
				t.Fatalf("Expected %v, got %v\n", tc.expectedReturn, returnValue)
			}
		})
	}
}

// Unit test for TestRetrieveRecent in regular_file_metadata_api.go
func TestRetrieveRecent(t *testing.T) {
	testcases := []struct {
		name           string
		entryInput     *MapEntry
		expectedReturn *MapEntryMetadata
	}{
		{
			name: "Succesful retrieval",
			entryInput: &MapEntry{
				ReferenceCount: 3,
				EntryList: map[uint64]*MapEntryMetadata{
					1: {},                                                    // default
					2: {},                                                    // default
					4: {},                                                    // default
					5: {Path: "one/we/want", Dev: 213, Ino: 54132, Gen: 324}, // The one we're hopeing to retrieve
				},
				IndexCounter:    5, // IndexCounter will ALWAYS be equal to or more than ReferenceCount
				UnderlyingInode: 123,
			},
			expectedReturn: &MapEntryMetadata{Path: "one/we/want", Dev: 213, Ino: 54132, Gen: 324},
		},
		{
			name:           "Empty MapEntry",
			entryInput:     &MapEntry{},
			expectedReturn: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			returnValue := RetrieveRecent(tc.entryInput)

			if !reflect.DeepEqual(tc.expectedReturn, returnValue) {
				t.Errorf("Expected %v, got %v\n", tc.expectedReturn, returnValue)
			}
		})
	}
}

// Unit test for LookupRegularFileMetadata in regular_file_metadata_api.go
func TestLookupRegularFileMetadata(t *testing.T) {
	testcases := []struct {
		name                        string
		inputHash                   [64]byte
		inputRef                    uint64
		regularFileMetadataHashStub map[[64]byte]*MapEntry
		expectedError               syscall.Errno
		expectedReturn              *MapEntryMetadata
	}{
		{
			name:      "Succesful lookup",
			inputHash: blake3Hash([]byte("test hash")),
			inputRef:  2,
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount: 4,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},                                           // misc entry
						2: {Path: "one/we/want", Dev: 123, Mode: 15431}, // entry we're retrieving
						3: {},
						4: {},
					},
				},
			},
			expectedError:  fs.OK,
			expectedReturn: &MapEntryMetadata{Path: "one/we/want", Dev: 123, Mode: 15431},
		},
		{
			name:      "Non existent MapEntryMetadata in existent MapEntry",
			inputHash: blake3Hash([]byte("test hash")),
			inputRef:  8,
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount: 4,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},                                           // misc entry
						2: {Path: "one/we/want", Dev: 123, Mode: 15431}, // entry we're retrieving
						3: {},
						4: {},
					},
				},
			},
			expectedError:  fs.ToErrno(syscall.ENODATA),
			expectedReturn: nil,
		},
		{
			name:      "Non existent MapEntry",
			inputHash: blake3Hash([]byte("doesn't exist!")),
			inputRef:  8,
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount: 4,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},                                           // misc entry
						2: {Path: "one/we/want", Dev: 123, Mode: 15431}, // entry we're retrieving
						3: {},
						4: {},
					},
				},
			},
			expectedError:  fs.ToErrno(syscall.ENODATA),
			expectedReturn: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			regularFileMetadataHash = tc.regularFileMetadataHashStub
			err, returnValue := LookupRegularFileMetadata(tc.inputHash, tc.inputRef)

			if err != tc.expectedError {
				t.Errorf("Expected %v, got %v\n", tc.expectedError, err)
			}
			if !reflect.DeepEqual(returnValue, tc.expectedReturn) {
				t.Errorf("Expected %v, got %v\n", tc.expectedReturn, returnValue)
			}
		})
	}
}

// Unit test for LookupRegularFileEntry in regular_file_metadata_api.go
func TestLookupRegularFileEntry(t *testing.T) {
	testcases := []struct {
		name                        string
		inputHash                   [64]byte
		regularFileMetadataHashStub map[[64]byte]*MapEntry
		expectedError               syscall.Errno
		expectedReturn              *MapEntry
	}{
		{
			name:      "Succesful lookup",
			inputHash: blake3Hash([]byte("test hash")),
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount:  1,
					EntryList:       map[uint64]*MapEntryMetadata{},
					IndexCounter:    123,
					UnderlyingInode: 485942,
				}, // One we want
				blake3Hash([]byte("foo bar")): {}, // Random misc entry
			},
			expectedError: fs.OK,
			expectedReturn: &MapEntry{
				ReferenceCount:  1,
				EntryList:       map[uint64]*MapEntryMetadata{},
				IndexCounter:    123,
				UnderlyingInode: 485942,
			},
		},
		{
			name:      "MapEntry doesn't exist",
			inputHash: blake3Hash([]byte("completely unrelated hash")),
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount:  1,
					EntryList:       map[uint64]*MapEntryMetadata{},
					IndexCounter:    123,
					UnderlyingInode: 485942,
				}, // One we want
				blake3Hash([]byte("foo bar")): {}, // Random misc entry
			},
			expectedError:  fs.ToErrno(syscall.ENODATA),
			expectedReturn: nil,
		},
		{
			name:                        "Empty regularFileMetadataHash",
			inputHash:                   blake3Hash([]byte("completely unrelated hash")),
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{}, // One we want
			expectedError:               fs.ToErrno(syscall.ENODATA),
			expectedReturn:              nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			regularFileMetadataHash = tc.regularFileMetadataHashStub

			err, returnValue := LookupRegularFileEntry(tc.inputHash)

			if err != tc.expectedError {
				t.Errorf("Expected %v, got %v\n", tc.expectedError, err)
			}
			if !reflect.DeepEqual(returnValue, tc.expectedReturn) {
				t.Errorf("Expected %v, got %v\n", tc.expectedReturn, returnValue)
			}
		})
	}

}

// Unit test for RemoveRegularFileMetadata in regular_file_metadata_api.go
func TestRemoveRegularFileMetadata(t *testing.T) {
	testcases := []struct {
		name                        string
		inputHash                   [64]byte
		inputRef                    uint64
		regularFileMetadataHashStub map[[64]byte]*MapEntry
		expectedError               syscall.Errno
		expectedState               map[[64]byte]*MapEntry
	}{
		{
			name:      "Succesful Deletion",
			inputHash: blake3Hash([]byte("test hash")),
			inputRef:  2,
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount: 4,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},                                           // misc entry
						2: {Path: "one/we/want", Dev: 123, Mode: 15431}, // entry we're deleting
						3: {},                                           // misc entry
						4: {},                                           // misc entry
					},
					IndexCounter:    4,
					UnderlyingInode: 123,
				},
			},
			expectedError: fs.OK,
			expectedState: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount: 3,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {}, // misc entry
						3: {},
						4: {},
					},
					IndexCounter:    4,
					UnderlyingInode: 123,
				},
			},
		},
		{
			name:      "Reference number doesn't exist",
			inputHash: blake3Hash([]byte("test hash")),
			inputRef:  6,
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount: 4,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},                                           // misc entry
						2: {Path: "one/we/want", Dev: 123, Mode: 15431}, // misc entry
						3: {},                                           // misc entry
						4: {},                                           // misc entry
					},
					IndexCounter:    4,
					UnderlyingInode: 123,
				},
			},
			expectedError: fs.ToErrno(syscall.ENODATA),
			expectedState: map[[64]byte]*MapEntry{ // Should be the same state
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount: 4,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},                                           // misc entry
						2: {Path: "one/we/want", Dev: 123, Mode: 15431}, // misc entry
						3: {},                                           // misc entry
						4: {},                                           // misc entry
					},
					IndexCounter:    4,
					UnderlyingInode: 123,
				},
			},
		},
		{
			name:      "Hash doesn't exist",
			inputHash: blake3Hash([]byte("non existing hash")),
			inputRef:  6,
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount: 4,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},                                           // misc entry
						2: {Path: "one/we/want", Dev: 123, Mode: 15431}, // misc entry
						3: {},                                           // misc entry
						4: {},                                           // misc entry
					},
					IndexCounter:    4,
					UnderlyingInode: 123,
				},
			},
			expectedError: fs.ToErrno(syscall.ENODATA),
			expectedState: map[[64]byte]*MapEntry{ // Should be the same state
				blake3Hash([]byte("random")): {}, // Random misc entry
				blake3Hash([]byte("test hash")): {
					ReferenceCount: 4,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},                                           // misc entry
						2: {Path: "one/we/want", Dev: 123, Mode: 15431}, // misc entry
						3: {},                                           // misc entry
						4: {},                                           // misc entry
					},
					IndexCounter:    4,
					UnderlyingInode: 123,
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			regularFileMetadataHash = tc.regularFileMetadataHashStub
			err := RemoveRegularFileMetadata(tc.inputHash, tc.inputRef)

			if err != tc.expectedError {
				t.Errorf("Expected %v, got %v\n", tc.expectedError, err)
			}
			if !reflect.DeepEqual(regularFileMetadataHash, tc.expectedState) {
				t.Errorf("Expected %v, got %v\n", tc.expectedState, regularFileMetadataHash)
			}
		})
	}
}

// Unit test for RetrieveRegularFileMapEntryFromHashAndRef in regular_file_metadata_api.go
func TestRetrieveRegularFileMapEntryFromHashAndRef(t *testing.T) {
	testcases := []struct {
		name                        string
		regularFileMetadataHashStub map[[64]byte]*MapEntry
		inputHash                   [64]byte
		inputRef                    uint64
		expectedReturn              *MapEntry
		expectedError               syscall.Errno
	}{
		{
			name: "Succesful Retrieval",
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random entry")): {},
				blake3Hash([]byte("one we want")): {
					ReferenceCount: 3,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},
						2: {Path: "Retrieve this one!", Mode: 132, Dev: 5},
						3: {},
					},
					IndexCounter:    5,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another random entry")): {},
			},
			inputHash: blake3Hash([]byte("one we want")),
			inputRef:  2,
			expectedReturn: &MapEntry{
				ReferenceCount: 3,
				EntryList: map[uint64]*MapEntryMetadata{
					1: {},
					2: {Path: "Retrieve this one!", Mode: 132, Dev: 5},
					3: {},
				},
				IndexCounter:    5,
				UnderlyingInode: 12345,
			},
			expectedError: fs.OK,
		},
		{
			name: "Non existing hash",
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random entry")): {},
				blake3Hash([]byte("one we want")): {
					ReferenceCount: 3,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},
						2: {Path: "Retrieve this one!", Mode: 132, Dev: 5},
						3: {},
					},
					IndexCounter:    5,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another random entry")): {},
			},
			inputHash:      blake3Hash([]byte("nonexisting hash")),
			inputRef:       2,
			expectedReturn: nil,
			expectedError:  fs.ToErrno(syscall.ENODATA),
		},
		{
			name: "Non existing refnum",
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random entry")): {},
				blake3Hash([]byte("one we want")): {
					ReferenceCount: 3,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},
						2: {Path: "Retrieve this one!", Mode: 132, Dev: 5},
						3: {},
					},
					IndexCounter:    5,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another random entry")): {},
			},
			inputHash:      blake3Hash([]byte("one we want")),
			inputRef:       7,
			expectedReturn: nil,
			expectedError:  fs.ToErrno(syscall.ENODATA),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			regularFileMetadataHash = tc.regularFileMetadataHashStub

			err, returnValue := RetrieveRegularFileMapEntryFromHashAndRef(tc.inputHash, tc.inputRef)

			if err != tc.expectedError {
				t.Errorf("Expected %v, got %v\n", tc.expectedError, err)
			}
			if !reflect.DeepEqual(returnValue, tc.expectedReturn) {
				t.Errorf("Expected %v, got %v\n", tc.expectedReturn, returnValue)
			}
		})
	}
}

// Unit test for RetrieveRegularFileMapEntryAndMetadataFromHashAndRef in regular_file_metadata_api.go
func TestRetrieveRegularFileMapEntryAndMetadataFromHashAndRef(t *testing.T) {
	testcases := []struct {
		name                        string
		regularFileMetadataHashStub map[[64]byte]*MapEntry
		inputHash                   [64]byte
		inputRef                    uint64
		expectedEntryReturn         *MapEntry
		expectedMetadataReturn      *MapEntryMetadata
		expectedError               syscall.Errno
	}{
		{
			name: "Succesful Retrieval",
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random entry")): {},
				blake3Hash([]byte("one we want")): {
					ReferenceCount: 3,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},
						2: {Path: "Retrieve this one!", Mode: 132, Dev: 5},
						3: {},
					},
					IndexCounter:    5,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another random entry")): {},
			},
			inputHash: blake3Hash([]byte("one we want")),
			inputRef:  2,
			expectedEntryReturn: &MapEntry{
				ReferenceCount: 3,
				EntryList: map[uint64]*MapEntryMetadata{
					1: {},
					2: {Path: "Retrieve this one!", Mode: 132, Dev: 5},
					3: {},
				},
				IndexCounter:    5,
				UnderlyingInode: 12345,
			},
			expectedMetadataReturn: &MapEntryMetadata{
				Path: "Retrieve this one!", Mode: 132, Dev: 5},
			expectedError: fs.OK,
		},
		{
			name: "Non existing hash",
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random entry")): {},
				blake3Hash([]byte("one we want")): {
					ReferenceCount: 3,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},
						2: {Path: "Retrieve this one!", Mode: 132, Dev: 5},
						3: {},
					},
					IndexCounter:    5,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another random entry")): {},
			},
			inputHash:              blake3Hash([]byte("nonexisting hash")),
			inputRef:               2,
			expectedEntryReturn:    nil,
			expectedMetadataReturn: nil,
			expectedError:          fs.ToErrno(syscall.ENODATA),
		},
		{
			name: "Non existing refnum",
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random entry")): {},
				blake3Hash([]byte("one we want")): {
					ReferenceCount: 3,
					EntryList: map[uint64]*MapEntryMetadata{
						1: {},
						2: {Path: "Retrieve this one!", Mode: 132, Dev: 5},
						3: {},
					},
					IndexCounter:    5,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another random entry")): {},
			},
			inputHash:              blake3Hash([]byte("one we want")),
			inputRef:               7,
			expectedEntryReturn:    nil,
			expectedMetadataReturn: nil,
			expectedError:          fs.ToErrno(syscall.ENODATA),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			regularFileMetadataHash = tc.regularFileMetadataHashStub

			err, returnEntryValue, returnMetadataValue := RetrieveRegularFileMapEntryAndMetadataFromHashAndRef(tc.inputHash, tc.inputRef)

			if err != tc.expectedError {
				t.Errorf("Expected %v, got %v\n", tc.expectedError, err)
			}
			if !reflect.DeepEqual(returnEntryValue, tc.expectedEntryReturn) {
				t.Errorf("Expected %v, got %v\n", tc.expectedEntryReturn, returnEntryValue)
			}
			if !reflect.DeepEqual(returnMetadataValue, tc.expectedMetadataReturn) {
				t.Errorf("Expected %v, got %v\n", tc.expectedMetadataReturn, returnMetadataValue)
			}
		})
	}
}

// Unit test for UpdateFullRegularFileMetadata in regular_file_metadata_api.go
func TestUpdateFullRegularFileMetadata(t *testing.T) {

	tmpDir := t.TempDir()

	testCases := []struct {
		name                        string
		filePath                    string
		inputUnstableAttr           *syscall.Stat_t
		inputStableAttr             *fs.StableAttr
		inputHash                   [64]byte
		inputRef                    uint64
		regularFileMetadataHashStub map[[64]byte]*MapEntry
		expectedError               syscall.Errno
		expectedReturn              map[[64]byte]*MapEntry
	}{
		{
			name:              "Succesful update",
			filePath:          tmpDir + "/file1",
			inputUnstableAttr: &syscall.Stat_t{},
			inputStableAttr:   &fs.StableAttr{Ino: 123456, Gen: 0, Mode: 0},
			inputHash:         blake3Hash([]byte("we want this one")),
			inputRef:          6,
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")):         {},
				blake3Hash([]byte("another random")): {},
				blake3Hash([]byte("we want this one")): {
					ReferenceCount: 7,
					EntryList: map[uint64]*MapEntryMetadata{
						1:  {},
						2:  {},
						4:  {},
						5:  {},
						6:  {},
						9:  {},
						12: {},
					},
					IndexCounter:    12,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another misc")): {},
			},
			expectedReturn: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")):         {},
				blake3Hash([]byte("another random")): {},
				blake3Hash([]byte("we want this one")): {
					ReferenceCount: 7,
					EntryList: map[uint64]*MapEntryMetadata{
						1:  {},
						2:  {},
						4:  {},
						5:  {},
						6:  {},
						9:  {},
						12: {},
					},
					IndexCounter:    12,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another misc")): {},
			},
			expectedError: fs.OK,
		},
		{
			name:              "Non-existent hash",
			filePath:          tmpDir + "/file2",
			inputUnstableAttr: &syscall.Stat_t{},
			inputStableAttr:   &fs.StableAttr{Ino: 123456, Gen: 0, Mode: 0},
			inputHash:         blake3Hash([]byte("doesn't exist")),
			inputRef:          6,
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")):         {},
				blake3Hash([]byte("another random")): {},
				blake3Hash([]byte("we want this one")): {
					ReferenceCount: 7,
					EntryList: map[uint64]*MapEntryMetadata{
						1:  {},
						2:  {},
						4:  {},
						5:  {},
						6:  {},
						9:  {},
						12: {},
					},
					IndexCounter:    12,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another misc")): {},
			},
			expectedReturn: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")):         {},
				blake3Hash([]byte("another random")): {},
				blake3Hash([]byte("we want this one")): {
					ReferenceCount: 7,
					EntryList: map[uint64]*MapEntryMetadata{
						1:  {},
						2:  {},
						4:  {},
						5:  {},
						6:  {},
						9:  {},
						12: {},
					},
					IndexCounter:    12,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another misc")): {},
			},
			expectedError: fs.ToErrno(syscall.ENODATA),
		},
		{
			name:              "Non-existent refnum",
			filePath:          tmpDir + "/file3",
			inputUnstableAttr: &syscall.Stat_t{},
			inputStableAttr:   &fs.StableAttr{Ino: 123456, Gen: 0, Mode: 0},
			inputHash:         blake3Hash([]byte("we want this one")),
			inputRef:          3,
			regularFileMetadataHashStub: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")):         {},
				blake3Hash([]byte("another random")): {},
				blake3Hash([]byte("we want this one")): {
					ReferenceCount: 7,
					EntryList: map[uint64]*MapEntryMetadata{
						1:  {},
						2:  {},
						4:  {},
						5:  {},
						6:  {},
						9:  {},
						12: {},
					},
					IndexCounter:    12,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another misc")): {},
			},
			expectedError: fs.ToErrno(syscall.ENODATA),
			expectedReturn: map[[64]byte]*MapEntry{
				blake3Hash([]byte("random")):         {},
				blake3Hash([]byte("another random")): {},
				blake3Hash([]byte("we want this one")): {
					ReferenceCount: 7,
					EntryList: map[uint64]*MapEntryMetadata{
						1:  {},
						2:  {},
						4:  {},
						5:  {},
						6:  {},
						9:  {},
						12: {},
					},
					IndexCounter:    12,
					UnderlyingInode: 12345,
				},
				blake3Hash([]byte("another misc")): {},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.filePath, func(t *testing.T) {

			regularFileMetadataHash = testCase.regularFileMetadataHashStub

			// Prepare file descriptor 'fd' ...
			fd, err := syscall.Creat(testCase.filePath, 666)
			if err != nil {
				t.Fatalf("Failed to create file - %v\n", err)
			}
			syscall.Fstat(fd, testCase.inputUnstableAttr)

			expected := generateExpectedMetadata(testCase.inputUnstableAttr, testCase.inputStableAttr, testCase.filePath)
			err2 := UpdateFullRegularFileMetadata(testCase.inputHash, testCase.inputRef, testCase.inputUnstableAttr, testCase.inputStableAttr, testCase.filePath)

			if err2 != testCase.expectedError {
				t.Errorf("Expected %v, got %v\n", testCase.expectedError, err2)
			}

			if testCase.expectedError == fs.OK {
				testCase.expectedReturn[testCase.inputHash].EntryList[testCase.inputRef] = expected
			}
			if !reflect.DeepEqual(testCase.expectedReturn, regularFileMetadataHash) {
				t.Errorf("Expected %v, got %v\n", testCase.expectedReturn, regularFileMetadataHash)
			}
		})
	}
}

// Unit tests for MigrateRegularFileMetadata in regular_file_metadata_api.go
func TestMigrateRegularFileMetadata(t *testing.T) {
	testcases := []struct {
		name              string
		inputOldMetadata  *MapEntryMetadata
		inputNewMetadata  *MapEntryMetadata
		inputUnstableAttr *syscall.Stat_t
		expectedReturn    *MapEntryMetadata
		expectedError     syscall.Errno
	}{
		{
			name: "Succesful update",
			inputOldMetadata: &MapEntryMetadata{
				Path:    "old/path",
				Mode:    9430,
				Dev:     5,
				Gen:     2,
				Ino:     823740,
				Uid:     1000,
				Gid:     1000,
				Rdev:    12,
				Size:    24353,
				Blocks:  44,
				Blksize: 4096,
				XAttr: map[string][]byte{
					"old": []byte("data"),
				},
				Nlink:     1,
				Atim:      syscall.Timespec{Sec: 12, Nsec: 43},
				Mtim:      syscall.Timespec{Sec: 12, Nsec: 43},
				Ctim:      syscall.Timespec{Sec: 2, Nsec: 3},
				X__pad0:   99,
				X__unused: [3]int64{0, 0, 0},
			},
			inputNewMetadata: &MapEntryMetadata{
				Path:      "new/path",
				Mode:      349,
				Dev:       5,
				Gen:       3,
				Ino:       823740,
				Uid:       1000,
				Gid:       1000,
				Rdev:      12,
				Size:      32454,
				Blocks:    48,
				Blksize:   4096,
				XAttr:     map[string][]byte{},
				Nlink:     1,
				Atim:      syscall.Timespec{Sec: 16, Nsec: 49},
				Mtim:      syscall.Timespec{Sec: 16, Nsec: 49},
				Ctim:      syscall.Timespec{Sec: 2, Nsec: 3},
				X__pad0:   99,
				X__unused: [3]int64{0, 0, 0},
			},
			inputUnstableAttr: &syscall.Stat_t{
				Dev:       21,
				Ino:       12435,
				Atim:      syscall.Timespec{Sec: 22, Nsec: 32},
				Mtim:      syscall.Timespec{Sec: 22, Nsec: 32},
				Ctim:      syscall.Timespec{Sec: 12, Nsec: 32},
				Nlink:     3,
				Uid:       123,
				Gid:       123,
				Mode:      534,
				Blksize:   3243,
				Blocks:    23,
				Size:      5432,
				Rdev:      1,
				X__unused: [3]int64{1, 2, 3},
				X__pad0:   32,
			},
			expectedError: fs.OK,
			expectedReturn: &MapEntryMetadata{
				Path: "old/path",
				Gen:  2,
				Mode: 9430,
				Ctim: syscall.Timespec{Sec: 2, Nsec: 3},
				Uid:  1000,
				Gid:  1000,
				Dev:  5,
				Ino:  823740,
				XAttr: map[string][]byte{
					"old": []byte("data"),
				},
				Atim:      syscall.Timespec{Sec: 22, Nsec: 32},
				Mtim:      syscall.Timespec{Sec: 22, Nsec: 32},
				Rdev:      1,
				Nlink:     3,
				Size:      5432,
				Blksize:   3243,
				Blocks:    23,
				X__unused: [3]int64{1, 2, 3},
				X__pad0:   32,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := MigrateRegularFileMetadata(tc.inputOldMetadata, tc.inputNewMetadata, tc.inputUnstableAttr)

			if err != tc.expectedError {
				t.Errorf("Expected %v, got %v\n", tc.expectedError, err)
			}
			if !reflect.DeepEqual(tc.inputNewMetadata, tc.expectedReturn) {
				t.Errorf("Expected %v, got %v\n", tc.expectedReturn, tc.inputNewMetadata)
			}
		})
	}
}
