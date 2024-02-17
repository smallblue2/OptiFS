package metadata

import (
	"bytes"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
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
			expectedErr: fs.ToErrno(syscall.EIO),
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
		name        string
		metadata    *MapEntryMetadata
		bufferSize  int // Size of the provided `dest` buffer
		expectedErr syscall.Errno
		expectedLen uint32
        expectedDest string
	}{
		{
			name:        "nil metadata",
			metadata:    nil,
			bufferSize:  100,
			expectedErr: fs.ToErrno(syscall.EIO),
			expectedLen: 0,
            expectedDest: "",
		},
		{
			name:        "empty xattrs",
			metadata:    &MapEntryMetadata{XAttr: map[string][]byte{}},
			bufferSize:  50,
			expectedErr: fs.OK,
			expectedLen: 0,
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
			bufferSize:  5,
			expectedErr: fs.ToErrno(syscall.ERANGE),
			expectedLen: 24, // Total length needed including null terminators
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
			bufferSize:  20,
			expectedErr: fs.OK,
			expectedLen: 16,
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
