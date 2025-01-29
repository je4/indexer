package main

import (
	"crypto/tls"
	"flag"
	"github.com/je4/filesystem/v3/pkg/vfsrw"
	"github.com/je4/filesystem/v3/pkg/writefs"
	"github.com/je4/indexer/v3/internal"
	"github.com/je4/indexer/v3/pkg/indexer"
	"github.com/je4/utils/v2/pkg/zLogger"
	ublogger "gitlab.switch.ch/ub-unibas/go-ublogger/v2"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
	"io"
	"io/fs"
	"log"
	"os"
)

var config = flag.String("config", "", "path to configuration toml file")

func main() {
	var err error
	flag.Parse()

	conf := LoadConfig(*config)

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

	var fss = map[string]fs.FS{"internal": internal.InternalFS}

	ad, err := indexer.InitActionDispatcher(fss, conf.Indexer, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot init action dispatcher")
	}

	vfs, err := vfsrw.NewFS(conf.VFS, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot create vfs")
	}
	defer func() {
		if err := vfs.Close(); err != nil {
			logger.Error().Err(err).Msg("cannot close vfs")
		}
	}()

	//	folder := "vfs://temp/10_3931_e-rara-119425_20231014T112407_master_ver1.zip/10_3931_e-rara-119425/image"
	//folder := "vfs://temp/BAU_1_004293301_20200720T123101_gen1_ver1.zip/004293301/fulltext"
	folder := "vfs://temp"
	filename := writefs.Join(vfs, folder, "M001BeataProgenies.xml")
	fileStat, err := fs.Stat(vfs, filename)
	if err != nil {
		logger.Fatal().Err(err).Msgf("cannot stat file %s", "vfs://temp/M001BeataProgenies.xml")
	}
	files := []fs.FileInfo{fileStat}
	/*
		files, err := fs.ReadDir(vfs, folder)
		if err != nil {
			logger.Fatal().Err(err).Msg("cannot read dir")
		}

	*/
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		logger.Info().Msgf("file: %s", file.Name())
		fp, err := vfs.Open(writefs.Join(vfs, folder, file.Name()))
		if err != nil {
			logger.Fatal().Err(err).Msgf("cannot open %s/%s", folder, file.Name())
		}
		result, err := ad.Stream(fp, []string{file.Name()}, []string{"siegfried", "xml", "identify", "ffprobe", "checksum"})
		if err != nil {
			fp.Close()
			logger.Fatal().Err(err).Msgf("cannot index %s/%s", folder, file.Name())
		}
		fp.Close()
		logger.Info().Msgf("%s::%s - %s - %s - w:%v h:%v - %v", result.Type, result.Subtype, result.Mimetype, result.Pronom, result.Width, result.Height, result.Checksum)
	}
}
