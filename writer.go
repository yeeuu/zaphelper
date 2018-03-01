package zaphelper

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
)

var (
	// ensure we always implement io.WriteCloser
	_ io.WriteCloser = (*Writer)(nil)
	// osStat exists so it can be mocked out by tests.
	osStat = os.Stat
)

// Writer is an io.WriteCloser that writes to the specified filename.
type Writer struct {
	// Filename is the file to write logs to.  Backup log files will be retained
	// in the same directory.  It uses <processname>-lumberjack.log in
	// os.TempDir() if empty.
	Filename string `json:"filename" yaml:"filename"`

	file *os.File
	mu   sync.Mutex
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		if err = w.openExistingOrNew(len(p)); err != nil {
			return 0, err
		}
	}

	n, err = w.file.Write(p)

	return n, err
}

// Close implements io.Closer, and closes the current logfile.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.close()
}

// close closes the file if it is open.
func (w *Writer) close() error {
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// Rotate causes Logger to close the existing log file and immediately create a
// new one.  This is a helper function for applications that want to initiate
// rotations outside of the normal rotation rules, such as in response to
// SIGHUP.
func (w *Writer) Rotate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotate()
}

// rotate closes the current file, moves it aside with a timestamp in the name,
// (if it exists), opens a new file with the original filename, and then runs
// post-rotation processing and removal.
func (w *Writer) rotate() error {
	if err := w.close(); err != nil {
		return errors.Wrap(err, "close old file failed.")
	}
	if err := w.openNew(); err != nil {
		return errors.Wrap(err, "open new file failed.")
	}
	return nil
}

// openNew opens a new log file for writing.
func (w *Writer) openNew() error {
	err := os.MkdirAll(w.dir(), 0744)
	if err != nil {
		return errors.Wrap(err, "can't make directories for new logfile")
	}

	name := w.filename()
	mode := os.FileMode(0644)

	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, mode)
	if err != nil {
		return errors.Wrap(err, "can't open new logfile")
	}
	w.file = f
	return nil
}

// openExistingOrNew opens the logfile if it exists and if the current write
// would not put it over MaxSize.  If there is no such file or the write would
// put it over the MaxSize, a new file is created.
func (w *Writer) openExistingOrNew(writeLen int) error {
	filename := w.filename()
	_, err := osStat(filename)
	if os.IsNotExist(err) {
		return w.openNew()
	}
	if err != nil {
		return errors.Wrap(err, "error getting log file info")
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		// if we fail to open the old log file for some reason, just ignore
		// it and open a new log file.
		return w.openNew()
	}
	w.file = file
	return nil
}

// genFilename generates the name of the logfile from the current time.
func (w *Writer) filename() string {
	if w.Filename != "" {
		return w.Filename
	}
	name := filepath.Base(os.Args[0]) + "-zap.log"
	return filepath.Join(os.TempDir(), name)
}

// dir returns the directory for the current filename.
func (w *Writer) dir() string {
	return filepath.Dir(w.filename())
}
