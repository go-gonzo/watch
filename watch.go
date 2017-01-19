package watch

import (
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/go-gonzo/fs/glob"
	"github.com/omeid/gonzo/context"
	"github.com/omeid/kargar"
	"gopkg.in/fsnotify.v1"
)

func throttle(limit time.Duration) func(func()) bool {
	var last int64
	lims := limit.Seconds()

	return func(cb func()) bool {
		now := time.Now().Unix()
		l := atomic.LoadInt64(&last)

		if l+int64(lims) < now {
			cb()
			atomic.StoreInt64(&last, now)
			return true
		}
		return false
	}
}

// Watcher ...
func Watcher(ctx context.Context, cb func(string), globs ...string) error {

	files, err := glob.Glob(globs...)

	if err != nil {
		return err
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	for matchpair := range files {
		w.Add(filepath.Dir(matchpair.Name))
		w.Add(matchpair.Name)
	}

	throttled := throttle(50 * time.Millisecond)
	go func() {
		for {
			select {
			case event := <-w.Events:
				ctx.Debugf("gonzo:watch:Event %v on %v", event, event.Name)
				//if event.Op&fsnotify.Write == fsnotify.Write {
				//event.Op&fsnotify.Create == fsnotify.Create ||
				throttled(func() {
					cb(event.Name)
				})
				//}
			case err := <-w.Errors:
				if err != nil {
					ctx.Error(err)
				}
			case <-ctx.Done():
				w.Close()
				return
			}
		}
	}()

	return nil
}

// WatcherSet is like watcher...
func WatcherSet(cb func(context.Context, ...string) error, watches map[string][]string) kargar.Action {
	return func(ctx context.Context) error {

		//function wrapper to copy set and files.
		for set, files := range watches {
			var s = set
			err := Watcher(
				ctx,
				func(string) { cb(ctx, s) },
				files...,
			)
			if err != nil {
				return err
			}
		}
		<-ctx.Done()
		return nil

	}
}
