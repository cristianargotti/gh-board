//go:build windows

package plan

import (
	"os"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// fileNameInfo is the FILE_INFO_BY_HANDLE_CLASS of a file name query.
const fileNameInfo = 2

var (
	kernel32                         = syscall.NewLazyDLL("kernel32.dll")
	procGetFileInformationByHandleEx = kernel32.NewProc("GetFileInformationByHandleEx")
)

// pipeName asks the handle for its name, which Git Bash and mintty ptys
// carry as named pipes.
func pipeName(f *os.File) (string, bool) {
	var buf [2 + syscall.MAX_PATH]uint16
	// The Windows API takes the buffer address; buf outlives the call.
	r, _, _ := procGetFileInformationByHandleEx.Call(f.Fd(), fileNameInfo, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2)) //nolint:gosec // audited above
	if r == 0 {
		return "", false
	}
	return fileNameOf(buf[:]), true
}

// fileNameOf decodes a FILE_NAME_INFO buffer: the byte length of the name
// in the first two UTF-16 units, then the name itself.
func fileNameOf(buf []uint16) string {
	length := int(buf[0]) | int(buf[1])<<16
	end := 2 + length/2
	if end > len(buf) {
		end = len(buf)
	}
	return string(utf16.Decode(buf[2:end]))
}
