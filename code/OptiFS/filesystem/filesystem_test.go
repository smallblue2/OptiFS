package filesystem

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// Helper function for mounting the filesystem
func mountFilesystem(t *testing.T) (mountpoint, underlying string, umountCmd *exec.Cmd) {
	// Creat temp directory for mountpoints
	mountpoint = t.TempDir()
	underlying = t.TempDir()

	// Start the filesystem
	exec.Command("filesystem", "-rm-persistence", mountpoint, underlying).Start()
	// Wait a second to be safe
	time.Sleep(1 * time.Second)

	// Build umount command
	umountCmd = exec.Command("umount", mountpoint)

	return
}

// Create an empty file with touch
func TestCreateEmptyFileWithTouch(t *testing.T) {
	testcases := []struct {
		name string
		file string
	}{
		{
			name: "Empty file",
			file: "testfile",
		},
	}

	mnt, und, stop := mountFilesystem(t)
	defer stop.Run()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			// Create a file with touch
			filePath := fmt.Sprintf("%v/%v", mnt, tc.file)
			log.Println(filePath)
			exec.Command("touch", filePath).Run()

            // Wait a second to be safe
            time.Sleep(1 * time.Second)

			// Ensure the file exists in OptiFS and underlying
			over := &syscall.Stat_t{}
			overLocation := fmt.Sprintf("%v/%v", mnt, tc.file)
			log.Println(overLocation)
			if err := syscall.Stat(overLocation, over); err != nil {
				t.Errorf("Couldn't stat file in OptiFS")
			}

            if over.Size != 0 {
                t.Error("Size of OptiFS node isn't 0\n")
            }

			under := &syscall.Stat_t{}
			underLocation := fmt.Sprintf("%v/%v", und, tc.file)
			log.Println(underLocation)
			if err := syscall.Stat(underLocation, under); err != nil {
				t.Errorf("Couldn't stat file in underlying filesystem")
			}

            if under.Size != 0 {
                t.Error("Size of underlying node isn't 0\n")
            }
		})
	}
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

	mnt, _, stop := mountFilesystem(t)
	defer stop.Run()

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
			log.Printf("Command: %v\n", cmd.String())
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

			// Ensure the file exists in OptiFS and underlying
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
