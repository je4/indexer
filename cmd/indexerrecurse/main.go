package main

import (
	"crypto/tls"
	_ "embed"
	"emperror.dev/errors"
	"flag"
	"fmt"
	"github.com/je4/filesystem/v3/pkg/zipasfolder"
	"github.com/je4/indexer/v3/pkg/util"
	"github.com/je4/trustutil/v2/pkg/loader"
	"github.com/je4/utils/v2/pkg/checksum"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	ublogger "gitlab.switch.ch/ub-unibas/go-ublogger"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed minimal.toml
var configToml []byte

var folder = flag.String("path", "", "path to iterate")

var waiter sync.WaitGroup

func worker(id int, fsys fs.FS, idx *util.Indexer, logger zLogger.ZLogger, jobs <-chan string, results chan<- string) {
	for path := range jobs {
		fmt.Println("worker", id, "processing job", path)
		r, cs, err := idx.Index(fsys, path, "", []string{"siegfried", "identify", "ffprobe", "tika", "xml"}, []checksum.DigestAlgorithm{checksum.DigestSHA512}, io.Discard, logger)
		if err != nil {
			logger.Error().Err(err).Msgf("cannot index (%s)%s", fsys, path)
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

	conf, err := util.LoadConfig(configToml)
	if err != nil {
		panic(fmt.Errorf("cannot load config: %v", err))
	}

	// create logger instance
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("cannot get hostname: %v", err)
	}

	var loggerTLSConfig *tls.Config
	var loggerLoader io.Closer
	if conf.Log.Stash.TLS != nil {
		loggerTLSConfig, loggerLoader, err = loader.CreateClientLoader(conf.Log.Stash.TLS, nil)
		if err != nil {
			log.Fatalf("cannot create client loader: %v", err)
		}
		defer loggerLoader.Close()
	}

	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	_logger, _logstash, _logfile := ublogger.CreateUbMultiLoggerTLS(conf.Log.Level, conf.Log.File,
		ublogger.SetDataset(conf.Log.Stash.Dataset),
		ublogger.SetLogStash(conf.Log.Stash.LogstashHost, conf.Log.Stash.LogstashPort, conf.Log.Stash.Namespace, conf.Log.Stash.LogstashTraceLevel),
		ublogger.SetTLS(conf.Log.Stash.TLS != nil),
		ublogger.SetTLSConfig(loggerTLSConfig),
	)
	if _logstash != nil {
		defer _logstash.Close()
	}

	if _logfile != nil {
		defer _logfile.Close()
	}

	l2 := _logger.With().Timestamp().Str("host", hostname).Logger() //.Output(output)
	var logger zLogger.ZLogger = &l2

	idx, err := util.InitIndexer(conf, logger)
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
	zipFS, err := zipasfolder.NewFS(dirFS, 10, logger)
	if err != nil {
		panic(fmt.Errorf("cannot create zip as folder FS: %v", err))
	}

	jobs := make(chan string, 100)
	results := make(chan string, 100)

	for w := 1; w <= 1; w++ {
		go worker(w, zipFS, idx, logger, jobs, results)
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
