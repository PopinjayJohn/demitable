// Command watcher is the phase-0 question 7 spike: does the watcher see
// everything, on every platform?
//
// It is disposable. Its answer lives in spikes/measurement/README.md §2.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PopinjayJohn/demitable/spikes/measurement/watcher"
	"github.com/fsnotify/fsnotify"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: watcher <command> [flags]")
		fmt.Fprintln(os.Stderr, "commands: rescan, atomic, fdcount, chmod, macos, limits")
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "rescan":
		cmdRescan(args)
	case "atomic":
		cmdAtomic(args)
	case "fdcount":
		cmdFDCount(args)
	case "chmod":
		cmdChmod(args)
	case "macos":
		cmdMacOS(args)
	case "limits":
		cmdLimits(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		os.Exit(2)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

// --- rescan -----------------------------------------------------------------

// cmdRescan measures the cost of a full scan, which is what sets the
// full-rescan interval.
func cmdRescan(args []string) {
	fs := flag.NewFlagSet("rescan", flag.ExitOnError)
	root := fs.String("root", "", "vault root")
	runs := fs.Int("runs", 5, "runs per measurement")
	fs.Parse(args)

	if *root == "" {
		fatal(fmt.Errorf("-root is required"))
	}
	files, err := watcher.WalkFiles(*root)
	if err != nil {
		fatal(err)
	}
	var bytes int64
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil {
			bytes += fi.Size()
		}
	}
	fmt.Printf("vault %s: %d notes, %d bytes\n", *root, len(files), bytes)

	// (size, mtime, inode) only.
	var statTimes []time.Duration
	for i := 0; i < *runs; i++ {
		cache := map[string]watcher.Entry{}
		res, err := watcher.Scan(*root, cache, false)
		if err != nil {
			fatal(err)
		}
		statTimes = append(statTimes, res.Duration)
	}
	// With a content hash.
	var hashTimes []time.Duration
	for i := 0; i < *runs; i++ {
		cache := map[string]watcher.Entry{}
		res, err := watcher.Scan(*root, cache, true)
		if err != nil {
			fatal(err)
		}
		hashTimes = append(hashTimes, res.Duration)
	}
	fmt.Printf("  scan, stat only      min %-12v median %-12v max %v\n", min(statTimes), median(statTimes), max(statTimes))
	fmt.Printf("  scan, stat + sha256  min %-12v median %-12v max %v\n", min(hashTimes), median(hashTimes), max(hashTimes))
	fmt.Printf("  per note: stat %v, stat+hash %v\n", median(statTimes)/time.Duration(len(files)), median(hashTimes)/time.Duration(len(files)))
	for _, iv := range []time.Duration{time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second} {
		fmt.Printf("  duty cycle at a %-6s interval: stat %.4f%%  stat+hash %.4f%%\n", iv,
			100*float64(median(statTimes))/float64(iv),
			100*float64(median(hashTimes))/float64(iv))
	}
}

func min(xs []time.Duration) time.Duration {
	s := xs[0]
	for _, x := range xs {
		if x < s {
			s = x
		}
	}
	return s
}

func max(xs []time.Duration) time.Duration {
	s := xs[0]
	for _, x := range xs {
		if x > s {
			s = x
		}
	}
	return s
}

func median(xs []time.Duration) time.Duration {
	s := append([]time.Duration{}, xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}

// --- atomic -----------------------------------------------------------------

// cmdAtomic is the acceptance test: a change made the way Obsidian makes
// one — write a temp file, rename it over the original — must be seen.
func cmdAtomic(args []string) {
	fs := flag.NewFlagSet("atomic", flag.ExitOnError)
	root := fs.String("root", "", "vault root")
	mode := fs.String("mode", "dir", "dir (watch directories) or file (watch the file itself)")
	fs.Parse(args)
	if *root == "" {
		fatal(fmt.Errorf("-root is required"))
	}

	note := filepath.Join(*root, "atomic-test.md")
	os.Remove(note)
	os.WriteFile(note, []byte("# Original\n\nthe original body\n"), 0o644)
	time.Sleep(50 * time.Millisecond)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		fatal(err)
	}
	defer w.Close()

	var watched []string
	if *mode == "file" {
		if err := w.Add(note); err != nil {
			fatal(err)
		}
		watched = []string{note}
	} else {
		if err := w.Add(*root); err != nil {
			fatal(err)
		}
		watched = []string{*root}
	}
	fmt.Printf("mode %-5s watching %v\n", *mode, watched)

	// Let the watcher settle.
	time.Sleep(100 * time.Millisecond)

	// The Obsidian-style save.
	newContent := []byte("# Original\n\nthe REPLACED body, written by a temp file and renamed over the top\n")
	savedAt := time.Now()
	if err := watcher.AtomicSave(note, newContent); err != nil {
		fatal(err)
	}

	deadline := time.After(2 * time.Second)
	sawTarget := false
	var events []string
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				break
			}
			lat := time.Since(savedAt)
			events = append(events, fmt.Sprintf("%-8s %-50s %v after the rename", ev.Op, ev.Name, lat.Round(time.Microsecond)))
			if ev.Name == note {
				sawTarget = true
				fmt.Printf("  SAW THE TARGET PATH after %v\n", lat.Round(time.Microsecond))
			}
		case <-deadline:
			goto done
		}
	}
done:
	for _, e := range events {
		fmt.Println("  event:", e)
	}
	if !sawTarget {
		fmt.Println("  RESULT: the change was NOT seen on the target path")
	} else {
		fmt.Println("  RESULT: the change WAS seen on the target path")
	}
	// Did the watcher survive? Read the file back.
	b, err := os.ReadFile(note)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("  content on disk after the save: %q\n", strings.TrimSpace(string(b)))

	// A second save, to show whether the watch survived the rename.
	fmt.Println("  second atomic save:")
	savedAt = time.Now()
	if err := watcher.AtomicSave(note, []byte("# Original\n\nthe SECOND replacement\n")); err != nil {
		fatal(err)
	}
	sawSecond := false
	deadline = time.After(1500 * time.Millisecond)
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				break
			}
			if ev.Name == note {
				sawSecond = true
				fmt.Printf("  SAW THE TARGET PATH again after %v\n", time.Since(savedAt).Round(time.Microsecond))
			}
		case <-deadline:
			goto done2
		}
	}
done2:
	if !sawSecond {
		fmt.Println("  RESULT: the second change was NOT seen — the watch was lost")
	} else {
		fmt.Println("  RESULT: the second change WAS seen — the watch survived")
	}
}

// --- fdcount ----------------------------------------------------------------

// cmdFDCount counts the file descriptors each strategy holds open.
func cmdFDCount(args []string) {
	fs := flag.NewFlagSet("fdcount", flag.ExitOnError)
	root := fs.String("root", "", "vault root")
	fs.Parse(args)
	if *root == "" {
		fatal(fmt.Errorf("-root is required"))
	}
	files, err := watcher.WalkFiles(*root)
	if err != nil {
		fatal(err)
	}
	dirs := map[string]bool{}
	for _, f := range files {
		dirs[filepath.Dir(f)] = true
	}

	base, err := watcher.CountFDs()
	if err != nil {
		fatal(err)
	}
	lim, _ := watcher.RLimitNoFile()
	fmt.Printf("vault %s: %d notes in %d directories\n", *root, len(files), len(dirs))
	fmt.Printf("open fds before watching: %d (RLIMIT_NOFILE %d)\n", base, lim)

	// The polling watcher: no fds at all beyond the ones it uses to walk.
	fmt.Println("\n-- the polling watcher --")
	before, _ := watcher.CountFDs()
	cache := map[string]watcher.Entry{}
	if _, err := watcher.Scan(*root, cache, true); err != nil {
		fatal(err)
	}
	after, _ := watcher.CountFDs()
	fmt.Printf("  fds before the scan %d, after %d (delta %+d) for %d notes\n", before, after, after-before, len(files))
	fmt.Printf("  fds per note: %.4f\n", float64(after-before)/float64(len(files)))

	// The fsnotify watcher: one watch per directory on Linux.
	fmt.Println("\n-- the fsnotify watcher (this platform's backend) --")
	w, err := fsnotify.NewWatcher()
	if err != nil {
		fatal(err)
	}
	defer w.Close()
	before, _ = watcher.CountFDs()
	for d := range dirs {
		if err := w.Add(d); err != nil {
			fatal(err)
		}
	}
	after, _ = watcher.CountFDs()
	fmt.Printf("  fds before %d, after adding %d directories %d (delta %+d) for %d notes\n",
		before, len(dirs), after, after-before, len(files))
	fmt.Printf("  fds per note: %.4f\n", float64(after-before)/float64(len(files)))
	fmt.Printf("  fds per directory: %.4f\n", float64(after-before)/float64(len(dirs)))

	// What happens if you watch the FILES instead of the directories.
	fmt.Println("\n-- the fsnotify watcher, watching files instead of directories --")
	w2, err := fsnotify.NewWatcher()
	if err != nil {
		fatal(err)
	}
	defer w2.Close()
	before, _ = watcher.CountFDs()
	added := 0
	for _, f := range files {
		if err := w2.Add(f); err == nil {
			added++
		}
	}
	after, _ = watcher.CountFDs()
	fmt.Printf("  fds before %d, after adding %d files %d (delta %+d)\n", before, added, after, after-before)
	fmt.Printf("  fds per note: %.4f\n", float64(after-before)/float64(len(files)))
}

// --- chmod ------------------------------------------------------------------

// cmdChmod reproduces the "REMOVE is reported as CHMOD" case.
func cmdChmod(args []string) {
	fs := flag.NewFlagSet("chmod", flag.ExitOnError)
	root := fs.String("root", "", "vault root")
	fs.Parse(args)
	if *root == "" {
		fatal(fmt.Errorf("-root is required"))
	}
	note := filepath.Join(*root, "chmod-test.md")
	os.WriteFile(note, []byte("# Chmod\n"), 0o644)
	time.Sleep(50 * time.Millisecond)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		fatal(err)
	}
	defer w.Close()
	if err := w.Add(*root); err != nil {
		fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	// Hold the file open, then delete it. Linux does not report REMOVE
	// until every descriptor is closed.
	f, err := os.Open(note)
	if err != nil {
		fatal(err)
	}
	fmt.Println("deleting a file that is open elsewhere:")
	os.Remove(note)
	time.Sleep(200 * time.Millisecond)
	// Drain what arrived.
	for {
		select {
		case ev := <-w.Events:
			fmt.Printf("  event while the file is still open: %-8s %s\n", ev.Op, filepath.Base(ev.Name))
		case <-time.After(300 * time.Millisecond):
			goto closed
		}
	}
closed:
	f.Close()
	fmt.Println("closing the descriptor:")
	deadline := time.After(1500 * time.Millisecond)
	for {
		select {
		case ev := <-w.Events:
			fmt.Printf("  event after the close: %-8s %s\n", ev.Op, filepath.Base(ev.Name))
		case <-deadline:
			goto done
		}
	}
done:
	// The reconciliation rule: on a Chmod, stat the path.
	if _, err := os.Stat(note); os.IsNotExist(err) {
		fmt.Println("  stat of the path says it is gone -> the watcher must treat this as a REMOVAL")
	}
}

// --- macos ------------------------------------------------------------------

// cmdMacOS is the test that has to be run on macOS. It is written so that
// the same binary runs there.
func cmdMacOS(args []string) {
	fs := flag.NewFlagSet("macos", flag.ExitOnError)
	root := fs.String("root", "", "vault root")
	fs.Parse(args)
	if *root == "" {
		fatal(fmt.Errorf("-root is required"))
	}
	fmt.Println("=== the macOS watcher acceptance test ===")
	fmt.Println("Run this on macOS with the same corpus. It asserts three things.")
	fmt.Println()
	files, err := watcher.WalkFiles(*root)
	if err != nil {
		fatal(err)
	}
	dirs := map[string]bool{}
	for _, f := range files {
		dirs[filepath.Dir(f)] = true
	}
	base, _ := watcher.CountFDs()
	lim, _ := watcher.RLimitNoFile()
	fmt.Printf("1. notes %d, directories %d, open fds %d, RLIMIT_NOFILE %d\n", len(files), len(dirs), base, lim)

	// The polling watcher.
	before, _ := watcher.CountFDs()
	cache := map[string]watcher.Entry{}
	if _, err := watcher.Scan(*root, cache, true); err != nil {
		fatal(err)
	}
	after, _ := watcher.CountFDs()
	fmt.Printf("2. polling watcher: fds %d -> %d (delta %+d), fds per note %.4f\n",
		before, after, after-before, float64(after-before)/float64(len(files)))

	// The fsnotify watcher, for comparison. On macOS this is kqueue and it
	// opens one fd per file.
	w, err := fsnotify.NewWatcher()
	if err != nil {
		fatal(err)
	}
	defer w.Close()
	before, _ = watcher.CountFDs()
	added := 0
	for d := range dirs {
		if err := w.Add(d); err == nil {
			added++
		}
	}
	after, _ = watcher.CountFDs()
	fmt.Printf("3. fsnotify (kqueue) watching %d directories: fds %d -> %d (delta %+d), fds per note %.4f\n",
		added, before, after, after-before, float64(after-before)/float64(len(files)))
	if float64(after-before)/float64(len(files)) > 0.5 {
		fmt.Println("   -> kqueue opens a descriptor per FILE. Do not use fsnotify on macOS.")
	} else {
		fmt.Println("   -> fsnotify did not open a descriptor per file on this platform.")
	}
}

// --- limits -----------------------------------------------------------------

func cmdLimits(args []string) {
	fs := flag.NewFlagSet("limits", flag.ExitOnError)
	fs.Parse(args)
	lim, err := watcher.RLimitNoFile()
	if err != nil {
		fatal(err)
	}
	fmt.Printf("RLIMIT_NOFILE (soft): %d\n", lim)
	for _, p := range []string{
		"/proc/sys/fs/inotify/max_user_watches",
		"/proc/sys/fs/inotify/max_user_instances",
		"/proc/sys/fs/inotify/max_queued_events",
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Printf("%s: (unavailable: %v)\n", p, err)
			continue
		}
		fmt.Printf("%s: %s", p, strings.TrimSpace(string(b)))
	}
}
