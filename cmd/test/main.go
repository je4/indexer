package main

import (
	"flag"
	"github.com/je4/filesystem/v2/pkg/osfsrw"
	"github.com/je4/filesystem/v2/pkg/zipasfolder"
	"github.com/je4/indexer/v2/internal"
	"github.com/je4/indexer/v2/pkg/indexer"
	lm "github.com/je4/utils/v2/pkg/logger"
	"io/fs"
	"path/filepath"
)

var config = flag.String("config", "", "path to configuration toml file")

func main() {
	flag.Parse()

	conf := LoadConfig(*config)

	// create logger instance
	logger, lf := lm.CreateLogger("indexer", conf.Logfile, nil, conf.Loglevel, conf.LogFormat)
	defer lf.Close()

	var fss = map[string]fs.FS{"internal": internal.InternalFS}

	ad, err := indexer.InitActionDispatcher(fss, conf.Indexer, logger)
	if err != nil {
		logger.Panicf("cannot init indexer: %v", err)
	}

	ofs, err := osfsrw.NewFS("c:/temp")
	if err != nil {
		logger.Panicf("cannot open osfsrw: %v", err)
	}
	zfs, err := zipasfolder.NewFS(ofs, 3)
	if err != nil {
		logger.Panicf("cannot open zipasfolder: %v", err)
	}

	folder := "10_3931_e-rara-100893_20221027T085444_master_ver1.zip/10_3931_e-rara-100893/image"
	files, err := fs.ReadDir(zfs, folder)
	if err != nil {
		logger.Panicf("cannot read dir: %v", err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		logger.Infof("file: %s", file.Name())
		fp, err := zfs.Open(filepath.Join(folder, file.Name()))
		if err != nil {
			logger.Panicf("cannot open %s/%s: %v", folder, file.Name(), err)
		}
		result, err := ad.Stream(fp, []string{file.Name()}, []string{"siegfried", "identify", "ffprobe", "checksum"})
		if err != nil {
			fp.Close()
			logger.Panicf("cannot index %s/%s: %v", folder, file.Name(), err)
		}
		fp.Close()
		logger.Infof("%s - %s - w:%v h:%v - %v", result.Mimetype, result.Pronom, result.Width, result.Height, result.Checksum)
	}
}
