package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/radovskyb/watcher"
)

const chmodMask watcher.Op = ^watcher.Op(0) ^ watcher.Chmod

// watch recursively watches changes in root and reports the filenames to names.
// It sends an error on the done chan.
// As an optimization, any dirs we encounter that meet the ExcludePrefix
// criteria of all reflexes can be ignored.
func watch(root string, fsmonitor *watcher.Watcher, names chan<- string, done chan<- error, reflexes []*Reflex) {
	if err := filepath.Walk(root, walker(fsmonitor, reflexes)); err != nil {
		infoPrintf(-1, "Error while walking path %s: %s", root, err)
	}

	for {
		select {
		case e := <-fsmonitor.Event:
			if verbose {
				infoPrintln(-1, "watcher event:", e)
			}
			if e.Op&chmodMask == 0 {
				continue
			}
			path := normalize(e.Name(), e.IsDir())
			names <- path
			if e.Op&watcher.Create > 0 && e.IsDir() {
				if err := filepath.Walk(path, walker(fsmonitor, reflexes)); err != nil {
					infoPrintf(-1, "Error while walking path %s: %s", path, err)
				}
			}
			// TODO: remove watches for deleted files
		case err := <-fsmonitor.Error:
			done <- err
			return
		}
	}
}

func walker(fsmonitor *watcher.Watcher, reflexes []*Reflex) filepath.WalkFunc {
	return func(path string, f os.FileInfo, err error) error {
		if err != nil || !f.IsDir() {
			return nil
		}
		path = normalize(path, f.IsDir())
		ignore := true
		for _, r := range reflexes {
			if !r.matcher.ExcludePrefix(path) {
				ignore = false
				break
			}
		}
		if ignore {
			return filepath.SkipDir
		}
		if err := fsmonitor.Add(path); err != nil {
			infoPrintf(-1, "Error while watching new path %s: %s", path, err)
		}
		return nil
	}
}

func normalize(path string, dir bool) string {
	path = strings.TrimPrefix(path, "./")
	if dir && !strings.HasSuffix(path, "/") {
		path = path + "/"
	}
	return path
}
