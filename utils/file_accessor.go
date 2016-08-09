package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"syscall"
)

// this helps with tesing

// doNotExit configures whether to exit on fatal errors or just panic
var doNotExit = false

type exitValue struct {
	Code   int
	Reason string
}

func (ev exitValue) Panic() {
	panic(ev)
}

func (ev exitValue) String() string {
	return fmt.Sprintf("code(%d): %s", ev.Code, ev.Reason)
}

func (ev exitValue) Exit() {
	if ev.Reason != "" {
		if ev.Code != 0 {
			fmt.Fprintln(os.Stderr, ev.Reason)
		} else {
			fmt.Println(ev.Reason)
		}
	}
	os.Exit(ev.Code)
}

// Exit allows exiting the process with a code while providing a reason
func Exit(code int, reason string, reasonArgs ...interface{}) {
	msg := fmt.Sprintf(reason, reasonArgs...)
	ev := exitValue{Code: code, Reason: msg}
	if doNotExit {
		ev.Panic()
	} else {
		ev.Exit()
	}
}

var (
	euid, egid, uid, gid  int
	defaultSecureAccessor = NewSecureFileAccessor()
)

// we define abort this way so we can mock it in the tests
var abort = func(msg string, args ...interface{}) {
	Exit(1, msg, args...)
	//	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	//	os.Exit(1)
}

func ensurePermissions(file string, maxPermissions os.FileMode) {
	mask := ^maxPermissions & 0777

	fileInfo, err := os.Stat(file)
	if err != nil {
		abort("Error checking permissions for %s: %s", file, err.Error())
	}
	fm := fileInfo.Mode()

	fileGID := int(fileInfo.Sys().(*syscall.Stat_t).Gid)
	fileUID := int(fileInfo.Sys().(*syscall.Stat_t).Uid)

	if fileUID != euid {
		abort("file '%s' must be owned by you.", file)
	}
	if maxPermissions&0077 != 0 && fileGID != egid {
		abort("file '%s' group must be yours", file)
	}

	if mask&fm != 0 {
		abort(
			"file '%s' must have permissions no more than %s; right now permissions are %s.",
			file, maxPermissions, fm,
		)
	}
}

// FileAccessor is the interface that defines the operations that file openers
// must support
type FileAccessor interface {
	ReadFile(file string) (string, error)
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	WriteFile(file, data string) error
}

//SecureFileAccessor allows defining the acceptable permission used when
// reading and writing files
type SecureFileAccessor struct {
	MaxPermissions          os.FileMode
	DefaultWritePermissions os.FileMode
}

// NewSecureFileAccessor returns a new SecureFileAccessor
func NewSecureFileAccessor() interface {
	FileAccessor
} {
	return &SecureFileAccessor{
		MaxPermissions:          os.FileMode(syscall.S_IRUSR | syscall.S_IWUSR | syscall.S_IXUSR),
		DefaultWritePermissions: os.FileMode(0644),
	}
}

// EnsurePermissions checks file and aborts the executing if it does not comply with the acceptable configured permissions
func (fa *SecureFileAccessor) EnsurePermissions(file string) {
	ensurePermissions(file, fa.MaxPermissions)
}

// OpenFile allows opening a file and ensuring its permissions are within the acceptable limits
func (fa *SecureFileAccessor) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {

	if perm&(^fa.MaxPermissions&0777) != 0 {
		abort("Requested opening file in a too permissive mode: '%s' (max permissions '%s')", perm, fa.MaxPermissions)
	}
	return os.OpenFile(name, flag, perm)
}

// ReadFile allows reading a file if it complies with the required permissions
func (fa *SecureFileAccessor) ReadFile(file string) (string, error) {
	fa.EnsurePermissions(file)
	bytes, err := ioutil.ReadFile(AbsFile(file))
	return string(bytes), err
}

// WriteFile allows writing a file and enforcing its permissions to the configured ones
func (fa *SecureFileAccessor) WriteFile(file, data string) error {
	err := ioutil.WriteFile(file, []byte(data), fa.DefaultWritePermissions)
	if err != nil {
		return err
	}
	return os.Chmod(file, fa.DefaultWritePermissions)
}

// OpenFileSecure allows opening a file ensure secure permissions
func OpenFileSecure(name string, flag int, perm os.FileMode) (*os.File, error) {
	if flag&syscall.O_CREAT != 0 && !FileExists(path.Dir(name)) {
		// Lets start restrictive...
		os.MkdirAll(path.Dir(name), 0700)
	}
	return defaultSecureAccessor.OpenFile(name, flag, perm)
}

// EnsurePermissions checks that the file permissions are less or equal
// than maxPermissions, aborting otherwise
func EnsurePermissions(file string, maxPermissions int) {
	if file != "" && FileExists(file) {
		ensurePermissions(file, os.FileMode(maxPermissions))
	}
}

// EnsureSafePermissions checks that file permissions are safe
// (less or equal than 0700), aborting otherwise
func EnsureSafePermissions(file string) {
	if file != "" && FileExists(file) {
		defaultSecureAccessor.(*SecureFileAccessor).EnsurePermissions(file)
	}
}

// WriteSecure writes data into file ensuring the resulting permissions are
// 0644 (only write permissions for the owner)
func WriteSecure(file, data string) error {
	return defaultSecureAccessor.WriteFile(file, data)
}

// ReadSecure returns a string containing the contents of file
// The function will cause the process to exit if any user other than the
// owner has write permissions over it
func ReadSecure(file string) (string, error) {
	return defaultSecureAccessor.ReadFile(file)
}

func init() {
	euid = os.Geteuid()
	egid = os.Getegid()
	uid = os.Getuid()
	gid = os.Getgid()
}
