package lumberjack

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// !!!NOTE!!!
//
// Running these tests in parallel will almost certainly cause sporadic (or even
// regular) failures, because they're all messing with the same global variable
// that controls the logic's mocked time.Now.  So... don't do that.

// Since all the tests uses the time to determine filenames etc, we need to
// control the wall clock as much as possible, which means having a wall clock
// that doesn't change unless we want it to.
var fakeCurrentTime = time.Now()

func fakeTime() time.Time {
	return fakeCurrentTime
}

func TestNewFile(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir(t, "TestNewFile")
	defer os.RemoveAll(dir)
	l := &Logger{
		Filename: logFile(dir),
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)
	existsWithContent(t, logFile(dir), b)
	fileCount(t, dir, 1)
}

func TestOpenExisting(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir(t, "TestOpenExisting")
	defer os.RemoveAll(dir)

	filename := logFile(dir)
	data := []byte("foo!")
	err := os.WriteFile(filename, data, fileModeNew)
	isNil(t, err)
	existsWithContent(t, filename, data)

	l := &Logger{
		Filename: filename,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	// Make sure the file got appended.
	existsWithContent(t, filename, append(data, b...))

	// Make sure no other files were created.
	fileCount(t, dir, 1)
}

func TestWriteTooLong(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir(t, "TestWriteTooLong")
	defer os.RemoveAll(dir)
	l := &Logger{
		Filename: logFile(dir),
		MaxBytes: 5,
	}
	defer l.Close()
	b := []byte("booooooooooooooo!")
	n, err := l.Write(b)
	notNil(t, err)
	equals(t, 0, n)
	equals(t, err.Error(),
		fmt.Sprintf("write length %d exceeds maximum file size %d", len(b), l.MaxBytes))
	_, err = os.Stat(logFile(dir))
	assert(t, os.IsNotExist(err), "File exists, but should not have been created")
}

func TestMakeLogDir(t *testing.T) {
	currentTime = fakeTime
	dir := time.Now().Format("TestMakeLogDir" + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	defer os.RemoveAll(dir)
	filename := logFile(dir)
	l := &Logger{
		Filename: filename,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)
	existsWithContent(t, logFile(dir), b)
	fileCount(t, dir, 1)
}

func TestDefaultFilename(t *testing.T) {
	currentTime = fakeTime
	dir := os.TempDir()
	filename := filepath.Join(dir, filepath.Base(os.Args[0])+"-lumberjack.log")
	defer os.Remove(filename)
	l := &Logger{}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)

	isNil(t, err)
	equals(t, len(b), n)
	existsWithContent(t, filename, b)
}

func TestAutoRotate(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir(t, "TestAutoRotate")
	defer os.RemoveAll(dir)

	filename := logFile(dir)
	l := &Logger{
		Filename: filename,
		MaxBytes: 10,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	existsWithContent(t, filename, b)
	fileCount(t, dir, 1)

	newFakeTime()

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	isNil(t, err)
	equals(t, len(b2), n)

	// the old logfile should be moved aside and the main logfile should have
	// only the last write in it.
	existsWithContent(t, filename, b2)

	// the backup file will use the current fake time and have the old contents.
	existsWithContent(t, backupFile(dir), b)

	fileCount(t, dir, 2)
}

func TestFirstWriteRotate(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir(t, "TestFirstWriteRotate")
	defer os.RemoveAll(dir)

	filename := logFile(dir)
	l := &Logger{
		Filename: filename,
		MaxBytes: 10,
	}
	defer l.Close()

	start := []byte("boooooo!")
	err := os.WriteFile(filename, start, 0o600)
	isNil(t, err)

	newFakeTime()

	// this would make us rotate
	b := []byte("fooo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	existsWithContent(t, filename, b)
	existsWithContent(t, backupFile(dir), start)

	fileCount(t, dir, 2)
}

func TestMaxBackups(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir(t, "TestMaxBackups")
	defer os.RemoveAll(dir)

	filename := logFile(dir)
	l := &Logger{
		Filename:   filename,
		MaxBytes:   10,
		MaxBackups: 1,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	existsWithContent(t, filename, b)
	fileCount(t, dir, 1)

	newFakeTime()

	// this will put us over the max
	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	isNil(t, err)
	equals(t, len(b2), n)

	// this will use the new fake time
	secondFilename := backupFile(dir)
	existsWithContent(t, secondFilename, b)

	// make sure the old file still exists with the same content.
	existsWithContent(t, filename, b2)

	fileCount(t, dir, 2)

	newFakeTime()

	// this will make us rotate again
	b3 := []byte("baaaaaar!")
	n, err = l.Write(b3)
	isNil(t, err)
	equals(t, len(b3), n)

	// this will use the new fake time
	thirdFilename := backupFile(dir)
	existsWithContent(t, thirdFilename, b2)

	existsWithContent(t, filename, b3)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(time.Millisecond * 10)

	// should only have two files in the dir still
	fileCount(t, dir, 2)

	// second file name should still exist
	existsWithContent(t, thirdFilename, b2)

	// should have deleted the first backup
	notExist(t, secondFilename)

	// now test that we don't delete directories or non-logfile files

	newFakeTime()

	// create a file that is close to but different from the logfile name.
	// It shouldn't get caught by our deletion filters.
	notlogfile := logFile(dir) + ".foo"
	err = os.WriteFile(notlogfile, []byte("data"), fileModeNew)
	isNil(t, err)

	// Make a directory that exactly matches our log file filters... it still
	// shouldn't get caught by the deletion filter since it's a directory.
	notlogfiledir := backupFile(dir)
	err = os.Mkdir(notlogfiledir, 0o700)
	isNil(t, err)

	newFakeTime()

	// this will use the new fake time
	fourthFilename := backupFile(dir)

	// Create a log file that is/was being compressed - this should
	// not be counted since both the compressed and the uncompressed
	// log files still exist.
	compLogFile := fourthFilename + compressSuffix
	err = os.WriteFile(compLogFile, []byte("compress"), fileModeNew)
	isNil(t, err)

	// this will make us rotate again
	b4 := []byte("baaaaaaz!")
	n, err = l.Write(b4)
	isNil(t, err)
	equals(t, len(b4), n)

	existsWithContent(t, fourthFilename, b3)
	existsWithContent(t, fourthFilename+compressSuffix, []byte("compress"))

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(time.Millisecond * 10)

	// We should have four things in the directory now - the 2 log files, the
	// not log file, and the directory
	fileCount(t, dir, 5)

	// third file name should still exist
	existsWithContent(t, filename, b4)

	existsWithContent(t, fourthFilename, b3)

	// should have deleted the first filename
	notExist(t, thirdFilename)

	// the not-a-logfile should still exist
	exists(t, notlogfile)

	// the directory
	exists(t, notlogfiledir)
}

func TestCleanupExistingBackups(t *testing.T) {
	// test that if we start with more backup files than we're supposed to have
	// in total, that extra ones get cleaned up when we rotate.

	currentTime = fakeTime

	dir := makeTempDir(t, "TestCleanupExistingBackups")
	defer os.RemoveAll(dir)

	// make 3 backup files

	data := []byte("data")
	backup := backupFile(dir)
	err := os.WriteFile(backup, data, fileModeNew)
	isNil(t, err)

	newFakeTime()

	backup = backupFile(dir)
	err = os.WriteFile(backup+compressSuffix, data, fileModeNew)
	isNil(t, err)

	newFakeTime()

	backup = backupFile(dir)
	err = os.WriteFile(backup, data, fileModeNew)
	isNil(t, err)

	// now create a primary log file with some data
	filename := logFile(dir)
	err = os.WriteFile(filename, data, fileModeNew)
	isNil(t, err)

	l := &Logger{
		Filename:   filename,
		MaxBytes:   10,
		MaxBackups: 1,
	}
	defer l.Close()

	newFakeTime()

	b2 := []byte("foooooo!")
	n, err := l.Write(b2)
	isNil(t, err)
	equals(t, len(b2), n)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(time.Millisecond * 10)

	// now we should only have 2 files left - the primary and one backup
	fileCount(t, dir, 2)
}

func TestMaxAge(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir(t, "TestMaxAge")
	defer os.RemoveAll(dir)

	filename := logFile(dir)
	l := &Logger{
		Filename: filename,
		MaxBytes: 10,
		MaxAge:   1,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	existsWithContent(t, filename, b)
	fileCount(t, dir, 1)

	// two days later
	newFakeTime()

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	isNil(t, err)
	equals(t, len(b2), n)
	existsWithContent(t, backupFile(dir), b)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// We should still have 2 log files, since the most recent backup was just
	// created.
	fileCount(t, dir, 2)

	existsWithContent(t, filename, b2)

	// we should have deleted the old file due to being too old
	existsWithContent(t, backupFile(dir), b)

	// two days later
	newFakeTime()

	b3 := []byte("baaaaar!")
	n, err = l.Write(b3)
	isNil(t, err)
	equals(t, len(b3), n)
	existsWithContent(t, backupFile(dir), b2)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// We should have 2 log files - the main log file, and the most recent
	// backup.  The earlier backup is past the cutoff and should be gone.
	fileCount(t, dir, 2)

	existsWithContent(t, filename, b3)

	// we should have deleted the old file due to being too old
	existsWithContent(t, backupFile(dir), b2)
}

func TestOldLogFiles(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1

	dir := makeTempDir(t, "TestOldLogFiles")
	defer os.RemoveAll(dir)

	filename := logFile(dir)
	data := []byte("data")
	err := os.WriteFile(filename, data, 0o7)
	isNil(t, err)

	// This gives us a time with the same precision as the time we get from the
	// timestamp in the name.
	t1, err := time.Parse(backupTimeFormat, fakeTime().UTC().Format(backupTimeFormat))
	isNil(t, err)

	backup := backupFile(dir)
	err = os.WriteFile(backup, data, 0o7)
	isNil(t, err)

	newFakeTime()

	t2, err := time.Parse(backupTimeFormat, fakeTime().UTC().Format(backupTimeFormat))
	isNil(t, err)

	backup2 := backupFile(dir)
	err = os.WriteFile(backup2, data, 0o7)
	isNil(t, err)

	l := &Logger{Filename: filename}
	files, err := l.oldLogFiles()
	isNil(t, err)
	equals(t, 2, len(files))

	// should be sorted by newest file first, which would be t2
	equals(t, t2, files[0].timestamp)
	equals(t, t1, files[1].timestamp)
}

func TestTimeFromName(t *testing.T) {
	l := &Logger{Filename: "/var/log/myfoo/foo.log"}
	prefix, ext := l.prefixAndExt()

	tests := []struct {
		filename string
		want     time.Time
		wantErr  bool
	}{
		{"foo-2014-05-04T14-44-33.555.log", time.Date(2014, 5, 4, 14, 44, 33, 555000000, time.UTC), false},
		{"foo-2014-05-04T14-44-33.555", time.Time{}, true},
		{"2014-05-04T14-44-33.555.log", time.Time{}, true},
		{"foo.log", time.Time{}, true},
	}

	for _, test := range tests {
		got, err := l.timeFromName(test.filename, prefix, ext)
		equals(t, got, test.want)
		equals(t, err != nil, test.wantErr)
	}
}

func TestLocalTime(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir(t, "TestLocalTime")
	defer os.RemoveAll(dir)

	l := &Logger{
		Filename:  logFile(dir),
		MaxBytes:  10,
		LocalTime: true,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	b2 := []byte("fooooooo!")
	n2, err := l.Write(b2)
	isNil(t, err)
	equals(t, len(b2), n2)

	existsWithContent(t, logFile(dir), b2)
	existsWithContent(t, backupFileLocal(dir), b)
}

func TestRotate(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir(t, "TestRotate")
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxBytes:   100,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	existsWithContent(t, filename, b)
	fileCount(t, dir, 1)

	newFakeTime()

	err = l.Rotate()
	isNil(t, err)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	filename2 := backupFile(dir)
	existsWithContent(t, filename2, b)
	existsWithContent(t, filename, []byte{})
	fileCount(t, dir, 2)
	newFakeTime()

	err = l.Rotate()
	isNil(t, err)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	filename3 := backupFile(dir)
	existsWithContent(t, filename3, []byte{})
	existsWithContent(t, filename, []byte{})
	fileCount(t, dir, 2)

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	isNil(t, err)
	equals(t, len(b2), n)

	// this will use the new fake time
	existsWithContent(t, filename, b2)
}

func TestCompressOnRotate(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir(t, "TestCompressOnRotate")
	defer os.RemoveAll(dir)

	filename := logFile(dir)
	l := &Logger{
		Compress: true,
		Filename: filename,
		MaxBytes: 10,
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(t, err)
	equals(t, len(b), n)

	existsWithContent(t, filename, b)
	fileCount(t, dir, 1)

	newFakeTime()

	err = l.Rotate()
	isNil(t, err)

	// the old logfile should be moved aside and the main logfile should have
	// nothing in it.
	existsWithContent(t, filename, []byte{})

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(300 * time.Millisecond)

	// a compressed version of the log file should now exist and the original
	// should have been removed.
	bc := new(bytes.Buffer)
	gz := gzip.NewWriter(bc)
	_, err = gz.Write(b)
	isNil(t, err)
	err = gz.Close()
	isNil(t, err)
	existsWithContent(t, backupFile(dir)+compressSuffix, bc.Bytes())
	notExist(t, backupFile(dir))

	fileCount(t, dir, 2)
}

func TestCompressOnResume(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir(t, "TestCompressOnResume")
	defer os.RemoveAll(dir)

	filename := logFile(dir)
	l := &Logger{
		Compress: true,
		Filename: filename,
		MaxBytes: 10,
	}
	defer l.Close()

	// Create a backup file and empty "compressed" file.
	filename2 := backupFile(dir)
	b := []byte("foo!")
	err := os.WriteFile(filename2, b, fileModeNew)
	isNil(t, err)
	err = os.WriteFile(filename2+compressSuffix, []byte{}, fileModeNew)
	isNil(t, err)

	newFakeTime()

	b2 := []byte("boo!")
	n, err := l.Write(b2)
	isNil(t, err)
	equals(t, len(b2), n)
	existsWithContent(t, filename, b2)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(300 * time.Millisecond)

	// The write should have started the compression - a compressed version of
	// the log file should now exist and the original should have been removed.
	bc := new(bytes.Buffer)
	gz := gzip.NewWriter(bc)
	_, err = gz.Write(b)
	isNil(t, err)
	err = gz.Close()
	isNil(t, err)
	existsWithContent(t, filename2+compressSuffix, bc.Bytes())
	notExist(t, filename2)

	fileCount(t, dir, 2)
}

func TestJson(t *testing.T) {
	data := []byte(`
{
	"filename": "foo",
	"maxbytes": 5,
	"maxage": 10,
	"maxbackups": 3,
	"localtime": true,
	"compress": true
}`[1:])

	l := Logger{}
	err := json.Unmarshal(data, &l)
	isNil(t, err)
	equals(t, "foo", l.Filename)
	equals(t, int64(5), l.MaxBytes)
	equals(t, 10, l.MaxAge)
	equals(t, 3, l.MaxBackups)
	equals(t, true, l.LocalTime)
	equals(t, true, l.Compress)
}

func TestYaml(t *testing.T) {
	data := []byte(`
filename: foo
maxbytes: 5
maxage: 10
maxbackups: 3
localtime: true
compress: true`[1:])

	l := Logger{}
	err := yaml.Unmarshal(data, &l)
	isNil(t, err)
	equals(t, "foo", l.Filename)
	equals(t, int64(5), l.MaxBytes)
	equals(t, 10, l.MaxAge)
	equals(t, 3, l.MaxBackups)
	equals(t, true, l.LocalTime)
	equals(t, true, l.Compress)
}

func TestToml(t *testing.T) {
	data := `
filename = "foo"
maxbytes = 5
maxage = 10
maxbackups = 3
localtime = true
compress = true`[1:]

	l := Logger{}
	md, err := toml.Decode(data, &l)
	isNil(t, err)
	equals(t, "foo", l.Filename)
	equals(t, int64(5), l.MaxBytes)
	equals(t, 10, l.MaxAge)
	equals(t, 3, l.MaxBackups)
	equals(t, true, l.LocalTime)
	equals(t, true, l.Compress)
	equals(t, 0, len(md.Undecoded()))
}

// makeTempDir creates a file with a semi-unique name in the OS temp directory.
// It should be based on the name of the test, to keep parallel tests from
// colliding, and must be cleaned up after the test is finished.
func makeTempDir(tb testing.TB, name string) string {
	tb.Helper()

	dir := time.Now().Format(name + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)

	isNilUp(tb, os.Mkdir(dir, 0o700))

	return dir
}

// existsWithContent checks that the given file exists and has the correct content.
func existsWithContent(tb testing.TB, path string, content []byte) {
	tb.Helper()

	info, err := os.Stat(path)
	isNilUp(tb, err)
	equalsUp(tb, int64(len(content)), info.Size())

	b, err := os.ReadFile(path)
	isNilUp(tb, err)
	equalsUp(tb, content, b)
}

// logFile returns the log file name in the given directory for the current fake
// time.
func logFile(dir string) string {
	return filepath.Join(dir, "foobar.log")
}

func backupFile(dir string) string {
	return filepath.Join(dir, "foobar-"+fakeTime().UTC().Format(backupTimeFormat)+".log")
}

func backupFileLocal(dir string) string {
	return filepath.Join(dir, "foobar-"+fakeTime().Format(backupTimeFormat)+".log")
}

// fileCount checks that the number of files in the directory is exp.
func fileCount(tb testing.TB, dir string, exp int) {
	tb.Helper()

	files, err := os.ReadDir(dir)
	isNilUp(tb, err)
	// Make sure no other files were created.
	equalsUp(tb, exp, len(files))
}

// newFakeTime sets the fake "current time" to two days later.
func newFakeTime() {
	fakeCurrentTime = fakeCurrentTime.Add(time.Hour * 24 * 2)
}

func notExist(tb testing.TB, path string) {
	tb.Helper()

	_, err := os.Stat(path)
	assertUp(tb, os.IsNotExist(err), 1, "expected to get os.IsNotExist, but instead got %v", err)
}

func exists(tb testing.TB, path string) {
	tb.Helper()

	_, err := os.Stat(path)
	assertUp(tb, err == nil, 1, "expected file to exist, but got error from os.Stat: %v", err)
}
