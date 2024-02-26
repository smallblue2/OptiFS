package permissions

import (
	"context"
	"filesystem/metadata"
	"os/user"
	"strconv"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

func Test_isOwner(t *testing.T) {
	testCases := []struct {
		name     string
		uid      uint32
		oUid     uint32
		expected bool
	}{
		{
			name:     "matchingUid",
			uid:      1000,
			oUid:     1000,
			expected: true,
		},
		{
			name:     "Non-matchingUid",
			uid:      1000,
			oUid:     0,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isOwner(tc.uid, tc.oUid)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}

}

func TestIsGroup(t *testing.T) {
	testCases := []struct {
		name     string
		gid      uint32
		oGid     uint32
		expected bool
	}{
		{
			name:     "matchingUid",
			gid:      1000,
			oGid:     1000,
			expected: true,
		},
		{
			name:     "matchingUid",
			gid:      1000,
			oGid:     0,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isGroup(tc.gid, tc.oGid)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}

}

func TestCheckMode(t *testing.T) {
	testCases := []struct {
		name            string
		uid             uint32
		gid             uint32
		nodeMetadata    *metadata.MapEntryMetadata
		ownerFlag       uint32
		groupFlag       uint32
		otherFlag       uint32
		currentSysadmin Sysadmin
		expected        bool
	}{
		{
			name:         "Owner can write",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 7892, Mode: 0b011101010},
			ownerFlag:    0b010000000, // owner write
			groupFlag:    0,
			otherFlag:    0,
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Other can read",
			uid:          1342,
			gid:          1980,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1232, Gid: 7819, Mode: 0b111100100},
			ownerFlag:    0,
			groupFlag:    0,
			otherFlag:    0b000000100, // other read
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Group can exec",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1320, Gid: 1000, Mode: 0b111101101},
			ownerFlag:    0,
			groupFlag:    0b000001000, // group exec
			otherFlag:    0,
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Owner can't exec",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1000, Mode: 0b110101101},
			ownerFlag:    0b001000000, // owner exec
			groupFlag:    0,
			otherFlag:    0,
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
		{
			name:         "Other can't write",
			uid:          1342,
			gid:          1980,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1232, Gid: 7819, Mode: 0b111100100},
			ownerFlag:    0,
			groupFlag:    0,
			otherFlag:    0b000000010, // other read
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
		{
			name:         "Group can't read",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1320, Gid: 1000, Mode: 0b111101001},
			ownerFlag:    0,
			groupFlag:    0b000000100, // group read
			otherFlag:    0,
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
		{
			name:         "Sysadmin Bypass",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1320, Gid: 1000, Mode: 0b111101001},
			ownerFlag:    0,
			groupFlag:    0b000000100, // group read
			otherFlag:    0,
			currentSysadmin: Sysadmin{
				UID: uintUID(getValidUID()),
				GID: uintGID(getValidGID()),
				Set: true,
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SysAdmin = tc.currentSysadmin
			result := checkMode(tc.uid, tc.gid, tc.nodeMetadata, tc.ownerFlag, tc.groupFlag, tc.otherFlag)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestReadCheck(t *testing.T) {
	testCases := []struct {
		name            string
		uid             uint32
		gid             uint32
		nodeMetadata    *metadata.MapEntryMetadata
		currentSysadmin Sysadmin
		expected        bool
	}{
		{
			name:         "Owner read",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1238, Mode: 0b111101101},
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Owner can't read",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1238, Mode: 0b011101101},
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
		{
			name:         "Sysadmin Bypass",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1238, Mode: 0b011101101},
			currentSysadmin: Sysadmin{
				UID: uintUID(getValidUID()),
				GID: uintGID(getValidGID()),
				Set: true,
			},
			expected: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SysAdmin = tc.currentSysadmin
			result := readCheck(tc.uid, tc.gid, tc.nodeMetadata)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestWriteCheck(t *testing.T) {
	testCases := []struct {
		name            string
		uid             uint32
		gid             uint32
		nodeMetadata    *metadata.MapEntryMetadata
		currentSysadmin Sysadmin
		expected        bool
	}{
		{
			name:         "Owner write",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1238, Mode: 0b111101101},
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Owner can't write",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1238, Mode: 0b101101101},
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
		{
			name:         "Sysadmin Bypass",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1238, Mode: 0b101101101},
			currentSysadmin: Sysadmin{
				UID: uintUID(getValidUID()),
				GID: uintGID(getValidGID()),
				Set: true,
			},
			expected: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SysAdmin = tc.currentSysadmin
			result := writeCheck(tc.uid, tc.gid, tc.nodeMetadata)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}

}

func TestExecCheck(t *testing.T) {
	testCases := []struct {
		name            string
		uid             uint32
		gid             uint32
		nodeMetadata    *metadata.MapEntryMetadata
		currentSysadmin Sysadmin
		expected        bool
	}{
		{
			name:         "Owner exec",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1238, Mode: 0b111101101},
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Owner can't exec",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1238, Mode: 0b110101101},
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
		{
			name:         "Sysadmin Bypass",
			uid:          1000,
			gid:          1000,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1238, Mode: 0b110101101},
			currentSysadmin: Sysadmin{
				UID: uintUID(getValidUID()),
				GID: uintGID(getValidGID()),
				Set: true,
			},
			expected: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SysAdmin = tc.currentSysadmin
			result := execCheck(tc.uid, tc.gid, tc.nodeMetadata)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestGetUIDGID(t *testing.T) {
	testCases := []struct {
		name          string
		ctx           context.Context
		expectedUid   uint32
		expectedGid   uint32
		expectedError syscall.Errno
	}{
		{
			name:          "Uid and Gid gotten",
			ctx:           fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			expectedUid:   1000,
			expectedGid:   1000,
			expectedError: fs.OK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			foundError, foundUid, foundGid := GetUIDGID(tc.ctx)
			if foundError != tc.expectedError {
				t.Errorf("Expected error %v, got error %v", tc.expectedError, foundError)
			}
			if foundError == fs.OK {
				if foundUid != tc.expectedUid || foundGid != tc.expectedGid {
					t.Errorf("Expected uid %v & gid %v, got uid %v & gid %v", tc.expectedUid, tc.expectedGid, foundUid, foundGid)
				}
			}
		})
	}
}

func TestCheckPermissions(t *testing.T) {
	testCases := []struct {
		name            string
		ctx             context.Context
		nodeMetadata    *metadata.MapEntryMetadata
		op              uint8
		currentSysadmin Sysadmin
		expected        bool
	}{
		{
			name:         "Check for Read Perms - Accepted",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1000, Mode: 0b100000000},
			op:           0, // read
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Check for Write Perms - Denied",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1000, Mode: 0b000000000},
			op:           1, // write
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SysAdmin = tc.currentSysadmin
			result := CheckPermissions(tc.ctx, tc.nodeMetadata, tc.op)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestCheckOpenIntent(t *testing.T) {
	testCases := []struct {
		name          string
		flags         uint32
		expectedRead  bool
		expectedWrite bool
	}{
		{
			name:          "Read Intent",
			flags:         syscall.O_RDONLY,
			expectedRead:  true,
			expectedWrite: false,
		},
		{
			name:          "Write Intent",
			flags:         syscall.O_WRONLY,
			expectedRead:  false,
			expectedWrite: true,
		},
		{
			name:          "Write Intent - Append",
			flags:         syscall.O_APPEND,
			expectedRead:  false,
			expectedWrite: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			foundRead, foundWrite := checkOpenIntent(tc.flags)
			if foundRead != tc.expectedRead || foundWrite != tc.expectedWrite {
				t.Errorf("Expected %v read, %v write, got %v read, %v write", tc.expectedRead, tc.expectedWrite, foundRead, foundWrite)
			}
		})
	}
}

func TestCheckOpenPermissions(t *testing.T) {
	testCases := []struct {
		name            string
		ctx             context.Context
		nodeMetadata    *metadata.MapEntryMetadata
		flags           uint32
		currentSysadmin Sysadmin
		expected        bool
	}{
		{
			name:         "Open intends to write - allowed",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1000, Mode: 0b111101101},
			flags:        syscall.O_WRONLY,
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Open intends to read - allowed",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1000, Mode: 0b111101101},
			flags:        syscall.O_RDONLY,
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Open intends to write - not allowed",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1000, Mode: 0b101101101},
			flags:        syscall.O_WRONLY,
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
		{
			name:         "Open intends to read - not allowed",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1000, Mode: 0b011101101},
			flags:        syscall.O_RDONLY,
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SysAdmin = tc.currentSysadmin
			result := CheckOpenPermissions(tc.ctx, tc.nodeMetadata, tc.flags)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestIsOwner(t *testing.T) {
	testCases := []struct {
		name         string
		ctx          context.Context
		nodeMetadata *metadata.MapEntryMetadata
		expected     bool
	}{
		{
			name:         "matchingUId",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1000},
			expected:     true,
		},
		{
			name:         "matchingUId",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1238, Gid: 1000},
			expected:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsOwner(tc.ctx, tc.nodeMetadata)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestCheckPermissionBits(t *testing.T) {
	testCases := []struct {
		name     string
		mask     uint32
		mode     uint32
		expected bool
	}{
		{
			name:     "Read allowed",
			mask:     4,
			mode:     syscall.S_IRUSR,
			expected: true,
		},
		{
			name:     "Write denied",
			mask:     2,
			mode:     0,
			expected: false,
		},
		{
			name:     "Read & execute",
			mask:     5,
			mode:     syscall.S_IRUSR | syscall.S_IXUSR,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := checkPermissionBits(tc.mask, tc.mode)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestCheckMask(t *testing.T) {
	testCases := []struct {
		name            string
		ctx             context.Context
		mask            uint32
		nodeMetadata    *metadata.MapEntryMetadata
		currentSysadmin Sysadmin
		expected        bool
	}{
		{
			name:         "Owner read - allowed",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			mask:         4,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1000, Gid: 1000, Mode: 0b111101101},
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: true,
		},
		{
			name:         "Group write - not allowed",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			mask:         2,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1210, Gid: 1000, Mode: 0b111101101},
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
		{
			name:         "Other execute - not allowed",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			mask:         2,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1210, Gid: 2875, Mode: 0b111101100},
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expected: false,
		},
		{
			name:         "Sysadmin Bypass",
			ctx:          fuse.NewContext(&fuse.Context{}, &fuse.Caller{Owner: fuse.Owner{Uid: 1000, Gid: 1000}, Pid: 123}),
			mask:         2,
			nodeMetadata: &metadata.MapEntryMetadata{Uid: 1210, Gid: 2875, Mode: 0b111101100},
			currentSysadmin: Sysadmin{
				UID: uintUID(getValidUID()),
				GID: uintGID(getValidGID()),
				Set: true,
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SysAdmin = tc.currentSysadmin
			result := CheckMask(tc.ctx, tc.mask, tc.nodeMetadata)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func getValidUID() string {
	user, err := user.Current()
	if err != nil {
		return ""
	}
	return user.Uid
}

func getValidGID() string {
	user, err := user.Current()
	if err != nil {
		return ""
	}
	return user.Gid
}

func uintUID(uid string) uint32 {
	converted, err := strconv.Atoi(uid)
	if err != nil {
		return 999
	}
	return uint32(converted)
}

func uintGID(gid string) uint32 {
	converted, err := strconv.Atoi(gid)
	if err != nil {
		return 999
	}
	return uint32(converted)
}

func TestValidUID(t *testing.T) {
	testCases := []struct {
		name     string
		uid      string
		expected bool
	}{
		{
			name:     "Valid UID check",
			uid:      getValidUID(),
			expected: true,
		},
		{
			name:     "Invalid UID check",
			uid:      "09248",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidUID(tc.uid)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestValidGID(t *testing.T) {
	testCases := []struct {
		name     string
		gid      string
		expected bool
	}{
		{
			name:     "Valid UID check",
			gid:      getValidGID(),
			expected: true,
		},
		{
			name:     "Invalid UID check",
			gid:      "09248",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidGID(tc.gid)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestChangeSysadminUID(t *testing.T) {
	testCases := []struct {
		name            string
		uid             string
		currentSysadmin Sysadmin
		expectedError   syscall.Errno
	}{
		{
			name: "Change sysadmin - valid",
			uid:  getValidUID(),
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expectedError: fs.OK,
		},
		{
			name: "Change sysadmin - invalid",
			uid:  "9248",
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expectedError: fs.ToErrno(syscall.ENOENT),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SysAdmin = tc.currentSysadmin
			err := ChangeSysadminUID(tc.uid)
			if err != tc.expectedError {
				t.Errorf("Expected %v. got %v\n", tc.expectedError, err)
			}
			if tc.expectedError == fs.OK {
				expected, err := strconv.Atoi(tc.uid)
				if err != nil {
					t.Fatal("Failed to convert UID string to int")
				}
				if SysAdmin.UID != uint32(expected) {
					t.Errorf("Expected %v, got %v\n", expected, SysAdmin.UID)
				}
			}
		})
	}
}

func TestChangeSysadminGID(t *testing.T) {
	testCases := []struct {
		name            string
		gid             string
		currentSysadmin Sysadmin
		expectedError   syscall.Errno
	}{
		{
			name: "Change sysadmin - valid",
			gid:  getValidGID(),
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expectedError: fs.OK,
		},
		{
			name: "Change sysadmin - invalid",
			gid:  "9248",
			currentSysadmin: Sysadmin{
				UID: 1320,
				GID: 1320,
				Set: true,
			},
			expectedError: fs.ToErrno(syscall.ENOENT),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SysAdmin = tc.currentSysadmin
			err := ChangeSysadminGID(tc.gid)
			if err != tc.expectedError {
				t.Errorf("Expected %v. got %v\n", tc.expectedError, err)
			}
			if tc.expectedError == fs.OK {
				expected, err := strconv.Atoi(getValidGID())
				if err != nil {
					t.Fatal("Failed to convert GID string to int")
				}
				if SysAdmin.GID != uint32(expected) {
					t.Errorf("Expected %v, got %v\n", expected, SysAdmin.GID)
				}
			}
		})
	}
}
