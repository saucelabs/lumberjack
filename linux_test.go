// +build linux

// +build linux

package lumberjack

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestMaintainMode(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir(t, "TestMaintainMode")
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	isNil(t, err)
	f.Close()

	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	newFakeTime()

	err = l.Rotate()
	isNil(t, err)

	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	isNil(t, err)
	info2, err := os.Stat(filename2)
	isNil(t, err)
	equals(t, mode, info.Mode())
	equals(t, mode, info2.Mode())
}

func TestMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	osChown = fakeFS.Chown
	osStat = fakeFS.Stat
	defer func() {
		osChown = os.Chown
		osStat = os.Stat
	}()
	currentTime = fakeTime
	dir := makeTempDir(t, "TestMaintainOwner")
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	isNil(t, err)
	f.Close()

	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	newFakeTime()

	err = l.Rotate()
	isNil(t, err)

	equals(t, 555, fakeFS.files[filename].uid)
	equals(t, 666, fakeFS.files[filename].gid)
}

func TestCompressMaintainMode(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir(t, "TestCompressMaintainMode")
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	isNil(t, err)
	f.Close()

	l := &Logger{
		Compress:   true,
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	newFakeTime()

	err = l.Rotate()
	isNil(t, err)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// mode.
	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	isNil(t, err)
	info2, err := os.Stat(filename2 + compressSuffix)
	isNil(t, err)
	equals(t, mode, info.Mode())
	equals(t, mode, info2.Mode())
}

func TestCompressMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	osChown = fakeFS.Chown
	osStat = fakeFS.Stat
	defer func() {
		osChown = os.Chown
		osStat = os.Stat
	}()
	currentTime = fakeTime
	dir := makeTempDir(t, "TestCompressMaintainOwner")
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	isNil(t, err)
	f.Close()

	l := &Logger{
		Compress:   true,
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	newFakeTime()

	err = l.Rotate()
	isNil(t, err)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// owner.
	filename2 := backupFile(dir)
	equals(t, 555, fakeFS.files[filename2+compressSuffix].uid)
	equals(t, 666, fakeFS.files[filename2+compressSuffix].gid)
}

type fakeFile struct {
	uid int
	gid int
}

type fakeFS struct {
	files map[string]fakeFile
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: make(map[string]fakeFile)}
}

func (fs *fakeFS) Chown(name string, uid, gid int) error {
	fs.files[name] = fakeFile{uid: uid, gid: gid}
	return nil
}

func (fs *fakeFS) Stat(name string) (os.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	stat.Uid = 555
	stat.Gid = 666
	return info, nil
}
