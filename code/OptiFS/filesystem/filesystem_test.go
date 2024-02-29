package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Helper function for mounting the filesystem
func mountFilesystem(t *testing.T) (mntPath, undPath string, cleanup func ()) {

    // Make directories to run OptiFS
    err1 := os.Mkdir("mountpoint", 0b111111111)
    if err1 != nil {
        t.Fatalf("Failed to create mountpoint dir")
    }

    err2 := os.Mkdir("underlying", 0b111111111)
    if err2 != nil {
        t.Fatalf("Failed to create underlying dir")
    }

    // Get the absolute path of both
    mntPath, err3 := filepath.Abs("mountpoint")
    if err3 != nil {
        t.Fatalf("Failed to get mountpoint dir abs path")
    }
    undPath, err4 := filepath.Abs("underlying")
    if err4 != nil {
        t.Fatalf("Failed to get underlying dir abs path")
    }

	// Start the filesystem
    cmd := exec.Command("filesystem", "-rm-persistence", mntPath, undPath)
    cmd.Start()

	// Wait a second to be safe
	time.Sleep(1 * time.Second)

	// Build revert function
    cleanup = func() {
        exec.Command("umount", mntPath).Run()
        os.RemoveAll(mntPath)
        os.RemoveAll(undPath)
        os.RemoveAll(fmt.Sprintf("%v/../save", undPath))
        time.Sleep(1 * time.Second)
    }

	return
}

// This tests the user creating file through the redirection of echo
func TestCreateFileWithEcho(t *testing.T) {
	testcases := []struct {
		name    string
		file    string
		content string
	}{
		{
			name:    "Empty file",
			file:    "testfile1",
			content: "",
		},
		{
			name:    "Non-empty file",
			file:    "testfile2",
			content: "this is a test",
		},
		{
			name:    "Larger file",
			file:    "testfile3",
			content: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Suspendisse tincidunt tincidunt odio a pretium. Morbi finibus justo a enim bibendum, in egestas lacus posuere. Mauris at lectus in tellus viverra finibus. Pellentesque fringilla elit quis vestibulum pretium. Morbi vestibulum leo at eros tempus, sit amet rhoncus nisi interdum. Morbi in sem pellentesque, mollis odio id, accumsan nisi. Aenean convallis ligula sed arcu pulvinar auctor. Etiam congue metus a accumsan placerat. Nam porttitor augue justo, in euismod libero ultricies a. Vivamus sollicitudin nunc est, id maximus lorem venenatis eget. Cras varius posuere diam vel placerat. Curabitur odio augue, tincidunt nec massa eu, feugiat convallis ante. Sed fringilla mattis justo, ac malesuada lectus placerat vel. Sed vulputate libero quis neque tempor, at commodo erat vestibulum.",
		},
	}

	mnt, _, cleanup := mountFilesystem(t)
	defer cleanup()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			// Create a file for echo redirection
			filePath := fmt.Sprintf("%v/%v", mnt, tc.file)
			file, err := os.Create(filePath)
			if err != nil {
				t.Errorf("Failed to create file to redirect echo to")
			}
			defer file.Close()

			cmd := exec.Command("echo", "-n", fmt.Sprintf("%v", tc.content))
			// Redirect stdout
			cmd.Stdout = file
			cmd.Run()

			// Ensure the file exists in OptiFS
			over := &syscall.Stat_t{}
			if err := syscall.Stat(filePath, over); err != nil {
				t.Errorf("File doesn't exist!")
			}

			// Ensure the file is the correct size
			length := int64(len([]byte(tc.content)))
			if over.Size != length {
				t.Errorf("Size is incorrect, expected %v, got %v\n", over.Size, length)
			}

			// Ensure the file content's are correct
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Errorf("Failed to read file: %v", err)
			}
			if string(content) != tc.content {
				t.Errorf("Expected {%v}, got {%v}", tc.content, string(content))
			}
		})
	}
}

// This tests the user creating duplicate files through the redirection of echo
func TestCreateDuplicateFilesWithEcho(t *testing.T) {
	testcases := []struct {
		name     string
		file1    string
		file2    string
		content1 string
		content2 string
	}{
		{
			name:     "Duplicate file - foobar",
			file1:    "testfile1",
			content1: "foobar",
			file2:    "testfile2",
			content2: "foobar",
		},
		{
			name:     "Duplicate file - longer",
			file1:    "testfile3",
			content1: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Suspendisse tincidunt tincidunt odio a pretium. Morbi finibus justo a enim bibendum, in egestas lacus posuere. Mauris at lectus in tellus viverra finibus. Pellentesque fringilla elit quis vestibulum pretium. Morbi vestibulum leo at eros tempus, sit amet rhoncus nisi interdum. Morbi in sem pellentesque, mollis odio id, accumsan nisi. Aenean convallis ligula sed arcu pulvinar auctor. Etiam congue metus a accumsan placerat. Nam porttitor augue justo, in euismod libero ultricies a. Vivamus sollicitudin nunc est, id maximus lorem venenatis eget. Cras varius posuere diam vel placerat. Curabitur odio augue, tincidunt nec massa eu, feugiat convallis ante. Sed fringilla mattis justo, ac malesuada lectus placerat vel. Sed vulputate libero quis neque tempor, at commodo erat vestibulum. ",
			file2:    "testfile4",
			content2: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Suspendisse tincidunt tincidunt odio a pretium. Morbi finibus justo a enim bibendum, in egestas lacus posuere. Mauris at lectus in tellus viverra finibus. Pellentesque fringilla elit quis vestibulum pretium. Morbi vestibulum leo at eros tempus, sit amet rhoncus nisi interdum. Morbi in sem pellentesque, mollis odio id, accumsan nisi. Aenean convallis ligula sed arcu pulvinar auctor. Etiam congue metus a accumsan placerat. Nam porttitor augue justo, in euismod libero ultricies a. Vivamus sollicitudin nunc est, id maximus lorem venenatis eget. Cras varius posuere diam vel placerat. Curabitur odio augue, tincidunt nec massa eu, feugiat convallis ante. Sed fringilla mattis justo, ac malesuada lectus placerat vel. Sed vulputate libero quis neque tempor, at commodo erat vestibulum. ",
		},
	}

	mnt, und, cleanup := mountFilesystem(t)
	defer cleanup()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			// Create files for echo redirection
			flags := os.O_WRONLY | os.O_CREATE

			filePath1 := fmt.Sprintf("%v/%v", mnt, tc.file1)
			fileWrite1, err1 := os.OpenFile(filePath1, flags, 0b110100100)
			if err1 != nil {
				t.Errorf("Failed to create file1 to redirect echo to")
			}
			filePath2 := fmt.Sprintf("%v/%v", mnt, tc.file2)
			fileWrite2, err2 := os.OpenFile(filePath2, flags, 0b110100100)
			if err2 != nil {
				t.Errorf("Failed to create file2 to redirect echo to")
			}

			// Fill files with Echo
			cmd1 := exec.Command("echo", "-n", fmt.Sprintf("%v", tc.content1))
			cmd1.Stdout = fileWrite1
			cmd1.Run()
			fileWrite1.Close()

			cmd2 := exec.Command("echo", "-n", fmt.Sprintf("%v", tc.content2))
			cmd2.Stdout = fileWrite2
			cmd2.Run()
			fileWrite2.Close()

			// Ensure the file exists in OptiFS
			stat1 := &syscall.Stat_t{}
			if err := syscall.Stat(filePath1, stat1); err != nil {
				t.Errorf("File1 doesn't exist!")
			}
			stat2 := &syscall.Stat_t{}
			if err := syscall.Stat(filePath2, stat2); err != nil {
				t.Errorf("File2 doesn't exist!")
			}

			// Ensure the files are the correct size
			length1 := int64(len([]byte(tc.content1)))
			if stat1.Size != length1 {
				t.Errorf("Size of file1 is incorrect, expected %v, got %v\n", stat1.Size, length1)
			}
			length2 := int64(len([]byte(tc.content2)))
			if stat2.Size != length2 {
				t.Errorf("Size of file2 is incorrect, expected %v, got %v\n", stat2.Size, length2)
			}

			// Ensure the files content are correct
			content1, err1 := os.ReadFile(filePath1)
			if err1 != nil {
				t.Errorf("Failed to read file1: %v", err1)
			}
			if string(content1) != tc.content1 {
				t.Errorf("Expected {%v}, got {%v}", tc.content1, string(content1))
			}
			content2, err2 := os.ReadFile(filePath2)
			if err2 != nil {
				t.Errorf("Failed to read file2: %v", err2)
			}
			if string(content2) != tc.content2 {
				t.Errorf("Expected {%v}, got {%v}", tc.content2, string(content2))
			}

			// Now check that the files are correctly linked underneath
			underlying1, underlying2 := &syscall.Stat_t{}, &syscall.Stat_t{}
			underErr1 := syscall.Stat(fmt.Sprintf("%v/%v", und, tc.file1), underlying1)

			if underErr1 != nil {
				t.Error("Couldn't stat underlying node1")
			}
			underErr2 := syscall.Stat(fmt.Sprintf("%v/%v", und, tc.file2), underlying2)
			if underErr2 != nil {
				t.Error("Couldn't stat underlying node2")
			}
			if underlying1.Nlink != 2 {
				t.Error("Incorrect link count on node1")
			}
			if underlying2.Nlink != 2 {
				t.Error("Incorrect link count on node2")
			}
		})
	}
}
