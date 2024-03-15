package main

import (
	_ "embed"
	"emperror.dev/errors"
	"flag"
	"fmt"
	"github.com/je4/filesystem/v2/pkg/zipasfolder"
	"github.com/je4/indexer/v2/pkg/util"
	"github.com/je4/utils/v2/pkg/checksum"
	lm "github.com/je4/utils/v2/pkg/logger"
	"github.com/je4/utils/v2/pkg/zLogger"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed minimal.toml
var configToml []byte

var folder = flag.String("path", "", "path to iterate")

var waiter sync.WaitGroup

func worker(id int, fsys fs.FS, idx *util.Indexer, logger zLogger.ZWrapper, jobs <-chan string, results chan<- string) {
	for path := range jobs {
		fmt.Println("worker", id, "processing job", path)
		r, cs, err := idx.Index(fsys, path, "", []string{"siegfried", "identify", "ffprobe", "tika"}, []checksum.DigestAlgorithm{checksum.DigestSHA512}, io.Discard, logger)
		if err != nil {
			logger.Errorf("cannot index (%s)%s: %v", fsys, path, err)
			waiter.Done()
			return
		}
		fmt.Printf("#%03d: %s/%s\n           [%s] - %s\n", id, fsys, path, r.Mimetype, cs[checksum.DigestSHA512])
		if r.Type == "image" {
			fmt.Printf("#           image: %vx%v", r.Width, r.Height)
		}
		results <- path + " done"
		waiter.Done()
	}
}

func main() {
	flag.Parse()

	// create logger instance
	log, lf := lm.CreateLogger(
		"indexerrecurse",
		"",
		nil,
		"DEBUG",
		`%{time:2006-01-02T15:04:05.000} %{shortpkg}::%{longfunc} [%{shortfile}] > %{level:.5s} - %{message}`)
	defer lf.Close()

	conf, err := util.LoadConfig(configToml)
	if err != nil {
		panic(fmt.Errorf("cannot load config: %v", err))
	}

	idx, err := util.InitIndexer(conf, log)
	if err != nil {
		panic(fmt.Errorf("cannot init indexer: %v", err))
	}

	if *folder == "" {
		*folder = "./"
	}
	if strings.HasPrefix(*folder, "./") {
		currDir, err := os.Getwd()
		if err != nil {
			panic(fmt.Errorf("cannot get working directory: %v", err))
		}
		*folder = filepath.Join(currDir, *folder)
	}
	dirFS := os.DirFS(*folder)
	zipFS, err := zipasfolder.NewFS(dirFS, 10)
	if err != nil {
		panic(fmt.Errorf("cannot create zip as folder FS: %v", err))
	}

	jobs := make(chan string, 100)
	results := make(chan string, 100)

	for w := 1; w <= 5; w++ {
		go worker(w, zipFS, idx, log, jobs, results)
	}

	go func() {
		for n := range results {
			fmt.Printf("%s\n", n)
		}
	}()

	if err := fs.WalkDir(zipFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.Wrapf(err, "cannot walk %s/%s", dirFS, path)
		}
		if d.IsDir() {
			fmt.Printf("[d] %s/%s\n", zipFS, path)
			return nil
		}
		fmt.Printf("[f] %s/%s\n", zipFS, path)
		isZip := strings.Contains(path, ".zip")
		if !isZip {
			//			return nil
		}

		waiter.Add(1)
		jobs <- path

		return nil
	}); err != nil {
		panic(fmt.Errorf("cannot walkd folder %v: %v", dirFS, err))
	}

	waiter.Wait()
	close(jobs)
}
