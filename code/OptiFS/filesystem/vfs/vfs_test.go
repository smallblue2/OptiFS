package vfs

import (
	"reflect"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
)

// Unit test for NewOptiFSFile in file.go
func TestNewOptiFSFile(t *testing.T) {
	testcases := []struct {
		name        string
		fdesc       int
		attr        fs.StableAttr
		flags       uint32
		currentHash [64]byte
		refNum      uint64
		expected    *OptiFSFile
	}{
		{
			name:        "Create ordinary OptiFSFile",
			fdesc:       12,
			attr:        fs.StableAttr{Mode: 511, Ino: 43314241997981142, Gen: 1},
			flags:       13,
			currentHash: [64]byte{32},
			refNum:      3,
			expected:    &OptiFSFile{fdesc: 12, attr: fs.StableAttr{Mode: 511, Ino: 43314241997981142, Gen: 1}, flags: 13, currentHash: [64]byte{32}, refNum: 3},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			returnFile := NewOptiFSFile(tc.fdesc, tc.attr, tc.flags, tc.currentHash, tc.refNum)

			if !reflect.DeepEqual(returnFile, tc.expected) {
				t.Errorf("Expected %+v, got %+v\n", tc.expected, returnFile)
			}
		})
	}
}

// Unit test for newNode in node.go
func TestNewNode(t *testing.T) {
	testcases := []struct {
		tname        string
		root         *OptiFSRoot
		parent       *fs.Inode
		name         string
		s            *syscall.Stat_t
		expectedNode fs.InodeEmbedder
	}{
		{
			tname:        "Ordinary node creation without custom function",
			root:         &OptiFSRoot{Path: "example/root", NewNode: nil},
			parent:       &fs.Inode{},
			name:         "testNode",
			s:            &syscall.Stat_t{},
			expectedNode: &OptiFSNode{RootNode: &OptiFSRoot{Path: "example/root", NewNode: nil}},
		},
		{
			tname: "Ordinary node creation with custom function",
			root: &OptiFSRoot{Path: "example/root", NewNode: func(data *OptiFSRoot, parent *fs.Inode, name string, s *syscall.Stat_t) fs.InodeEmbedder {
				return &OptiFSNode{
					RootNode: &OptiFSRoot{Path: "other/root", NewNode: nil}}
			}},
			parent:       &fs.Inode{},
			name:         "testNode",
			s:            &syscall.Stat_t{},
			expectedNode: &OptiFSNode{RootNode: &OptiFSRoot{Path: "other/root", NewNode: nil}},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.tname, func(t *testing.T) {
			returnNode := tc.root.newNode(tc.parent, tc.name, tc.s)

			if !reflect.DeepEqual(returnNode, tc.expectedNode) {
				t.Errorf("Expected %+v, got %+v\n", tc.expectedNode, returnNode)
			}

		})
	}
}
