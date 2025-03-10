package main

import (
	"crypto/tls"
	_ "embed"
	"emperror.dev/errors"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/je4/indexer/v3/pkg/indexer"
	"github.com/je4/indexer/v3/pkg/util"
	"github.com/je4/utils/v2/pkg/checksum"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	ublogger "gitlab.switch.ch/ub-unibas/go-ublogger/v2"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

//go:embed minimal.toml
var configToml []byte

var folder = flag.String("path", "", "path to iterate")
var jsonFlag = flag.String("json", "", "json file to write")
var csvFlag = flag.String("csv", "", "csv file to write")
var concurrentFlag = flag.Uint("n", 3, "number of concurrent workers")
var actionsFlag = flag.String("actions", "", "comma separated actions to perform")

var waiter sync.WaitGroup

var serialWriterLock sync.Mutex

func serialWriteLine(w io.Writer, d []byte) error {
	serialWriterLock.Lock()
	defer serialWriterLock.Unlock()
	if _, err := w.Write(append(d, []byte("\n")...)); err != nil {
		return errors.Wrapf(err, "cannot write to output")
	}
	return nil
}

func worker(id uint, fsys fs.FS, idx *util.Indexer, logger zLogger.ZLogger, jobs <-chan string, results chan<- string, jsonlWriter io.Writer, csvWriter *csv.Writer) {
	for path := range jobs {
		fmt.Println("worker", id, "processing job", path)
		actions := []string{"siegfried", "xml"} // , "identify", "ffprobe", "tika"
		if *actionsFlag != "" {
			for _, a := range strings.Split(*actionsFlag, ",") {
				a = strings.ToLower(strings.TrimSpace(a))
				if a != "" {
					actions = append(actions, a)
				}
			}
		}
		slices.Sort(actions)
		actions = slices.Compact(actions)
		r, cs, err := idx.Index(fsys, path, "", actions, []checksum.DigestAlgorithm{checksum.DigestSHA512}, io.Discard, logger)
		if err != nil {
			logger.Error().Err(err).Msgf("cannot index (%s)%s", fsys, path)
			waiter.Done()
			return
		}
		fmt.Printf("#%03d: %s/%s\n           [%s] - %s\n", id, fsys, path, r.Mimetype, cs[checksum.DigestSHA512])
		if r.Type == "image" {
			fmt.Printf("#           image: %vx%v", r.Width, r.Height)
		}
		if jsonlWriter != nil {
			outStruct := struct {
				Path     string            `json:"path"`
				Folder   string            `json:"folder"`
				Basename string            `json:"basename"`
				Indexer  *indexer.ResultV2 `json:"indexer"`
			}{path, filepath.Dir(path), filepath.Base(path), r}
			data, err := json.Marshal(outStruct)
			if err != nil {
				logger.Error().Err(err).Msgf("cannot marshal result")
			} else {
				if err := serialWriteLine(jsonlWriter, data); err != nil {
					logger.Error().Err(err).Msgf("cannot write to output")
				}
			}
		}
		if csvWriter != nil {
			csvWriter.Write([]string{path, filepath.Dir(path), filepath.Base(path), fmt.Sprintf("%v", r.Size), r.Mimetype, r.Pronom, r.Type, r.Subtype, cs[checksum.DigestSHA512], fmt.Sprintf("%v", r.Width), fmt.Sprintf("%v", r.Height), fmt.Sprintf("%v", r.Duration)})
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

	var jsonlOutfile io.WriteCloser
	if *jsonFlag != "" {
		jsonlOutfile, err = os.Create(*jsonFlag)
		if err != nil {
			log.Fatalf("cannot create json file: %v", err)
		}
		defer jsonlOutfile.Close()
	}
	var csvOutfile io.WriteCloser
	var csvWriter *csv.Writer
	if *csvFlag != "" {
		csvOutfile, err = os.Create(*csvFlag)
		if err != nil {
			log.Fatalf("cannot create csv file: %v", err)
		}
		defer csvOutfile.Close()

		csvWriter = csv.NewWriter(csvOutfile)
		defer csvWriter.Flush()
		csvWriter.Write([]string{"path", "folder", "basename", "size", "mimetype", "pronom", "type", "subtype", "checksum", "width", "height", "duration"})
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
	_logger, _logstash, _logfile, err := ublogger.CreateUbMultiLoggerTLS(conf.Log.Level, conf.Log.File,
		ublogger.SetDataset(conf.Log.Stash.Dataset),
		ublogger.SetLogStash(conf.Log.Stash.LogstashHost, conf.Log.Stash.LogstashPort, conf.Log.Stash.Namespace, conf.Log.Stash.LogstashTraceLevel),
		ublogger.SetTLS(conf.Log.Stash.TLS != nil),
		ublogger.SetTLSConfig(loggerTLSConfig),
	)
	if err != nil {
		log.Fatalf("cannot create logger: %v", err)
	}
	if _logstash != nil {
		defer _logstash.Close()
	}

	if _logfile != nil {
		defer _logfile.Close()
	}

	l2 := _logger.With().Timestamp().Str("host", hostname).Logger() //.Output(output)
	var logger zLogger.ZLogger = &l2

	idx, err := util.InitIndexer(conf.Indexer, logger)
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
	//zipFS, err := zipasfolder.NewFS(dirFS, 10, true, logger)
	zipFS := dirFS
	if err != nil {
		panic(fmt.Errorf("cannot create zip as folder FS: %v", err))
	}

	jobs := make(chan string, 100)
	results := make(chan string, 100)

	for w := uint(1); w <= *concurrentFlag; w++ {
		go worker(w, zipFS, idx, logger, jobs, results, jsonlOutfile, csvWriter)
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
