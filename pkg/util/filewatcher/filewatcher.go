package filewatcher

import (
	"io/ioutil"
	"os"
	"time"

	"k8s.io/klog"
)

// Watches a file on the local filesystem pointed to by path. This
// will refresh the cached file contents periodically if the file has
// changed. CheckPeriod serves to ensure that we don't stat the file
// incessantly.
type Watcher interface {
	Contents() string
	Version() int
}

type File struct {
	path         string
	modTime      time.Time
	fileContents string
	statTime     time.Time
	CheckPeriod  time.Duration
	version      int
}

func New(path string) *File {
	fw := &File{
		path:        path,
		CheckPeriod: 10 * time.Second,
		version:     1,
	}
	fw.refresh()
	return fw
}

func (fw *File) Version() int {
	return fw.version
}

func (fw *File) refresh() (changed bool) {
	changed = false
	if fw.path == "" {
		return changed
	}
	now := time.Now()
	if fw.statTime.Add(fw.CheckPeriod).Before(now) {
		info, err := os.Stat(fw.path)
		if err != nil {
			klog.Warningf("Error getting file info at %s: %s", fw.path, err)
		}
		if info.ModTime().After(fw.modTime) {
			c, err := ioutil.ReadFile(fw.path)
			if err != nil {
				klog.Warningf("Error reading contents of file at %s: %s", fw.path, err)
				return
			}
			changed = true
			fw.version += 1
			fw.fileContents = string(c)
			fw.modTime = info.ModTime()
		}
		fw.statTime = now
	}
	return changed
}

func (fw *File) Contents() string {
	fw.refresh()
	return fw.fileContents
}
