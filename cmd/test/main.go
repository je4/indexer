package main

import (
	"emperror.dev/errors"
	"flag"
	"github.com/je4/filesystem/v2/pkg/osfsrw"
	"github.com/je4/filesystem/v2/pkg/zipasfolder"
	datasiegfried "github.com/je4/indexer/v2/data/siegfried"
	indexerConfig "github.com/je4/indexer/v2/pkg/config"
	"github.com/je4/indexer/v2/pkg/indexer"
	ironmaiden "github.com/je4/indexer/v2/pkg/indexer"
	lm "github.com/je4/utils/v2/pkg/logger"
	"github.com/op/go-logging"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

var config = flag.String("config", "", "path to configuration toml file")

func stringMapToMimeRelevance(mimeRelevanceInterface map[string]indexerConfig.MimeWeight) (map[int]ironmaiden.MimeWeightString, error) {
	var mimeRelevance = map[int]ironmaiden.MimeWeightString{}
	for keyStr, val := range mimeRelevanceInterface {
		key, err := strconv.Atoi(keyStr)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid key entry '%s' in 'MimeRelevance'", keyStr)
		}
		mimeRelevance[key] = ironmaiden.MimeWeightString{
			Regexp: val.Regexp,
			Weight: val.Weight,
		}
	}
	return mimeRelevance, nil
}

func InitActionDispatcher(conf indexerConfig.Indexer, logger *logging.Logger) (*indexer.ActionDispatcher, error) {
	mimeRelevance, err := stringMapToMimeRelevance(conf.MimeRelevance)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert config string map to mime relevance")
	}
	actionDispatcher := ironmaiden.NewActionDispatcher(mimeRelevance)
	signatureData, err := os.ReadFile(conf.Siegfried.SignatureFile)
	if err != nil {
		logger.Warningf("no siegfried signature file provided. using default signature file. please provide a recent signature file.")
		signatureData = datasiegfried.DefaultSig
	}
	_ = ironmaiden.NewActionSiegfried(
		"siegfried",
		signatureData,
		conf.Siegfried.MimeMap,
		nil,
		actionDispatcher)
	logger.Info("indexer action siegfried added")

	if conf.Checksum.Enabled {
		_ = ironmaiden.NewActionChecksum(
			"checksum",
			conf.Checksum.Digest,
			nil,
			actionDispatcher,
		)
	}
	if conf.FFMPEG.Enabled {
		_ = ironmaiden.NewActionFFProbe(
			"ffprobe",
			conf.FFMPEG.FFProbe,
			conf.FFMPEG.Wsl,
			conf.FFMPEG.Timeout.Duration,
			conf.FFMPEG.Online,
			conf.FFMPEG.Mime,
			nil,
			actionDispatcher)
		logger.Info("indexer action ffprobe added")
	}
	if conf.ImageMagick.Enabled {
		_ = ironmaiden.NewActionIdentifyV2("identify",
			conf.ImageMagick.Identify,
			conf.ImageMagick.Convert,
			conf.ImageMagick.Wsl,
			conf.ImageMagick.Timeout.Duration,
			conf.ImageMagick.Online, nil, actionDispatcher)
		logger.Info("indexer action identify added")
	}
	if conf.Tika.Enabled {
		_ = ironmaiden.NewActionTika("tika",
			conf.Tika.AddressMeta,
			conf.Tika.Timeout.Duration,
			conf.Tika.RegexpMimeMeta,
			conf.Tika.RegexpMimeMetaNot,
			"",
			conf.Tika.Online,
			nil, actionDispatcher)
		logger.Info("indexer action tika added")

		_ = ironmaiden.NewActionTika("fulltext",
			conf.Tika.AddressFulltext,
			conf.Tika.Timeout.Duration,
			conf.Tika.RegexpMimeFulltext,
			conf.Tika.RegexpMimeFulltextNot,
			"X-TIKA:content",
			conf.Tika.Online,
			nil,
			actionDispatcher)
		logger.Info("indexer action fulltext added")

	}

	return actionDispatcher, nil
}

func main() {
	flag.Parse()

	conf := LoadConfig(*config)

	// create logger instance
	logger, lf := lm.CreateLogger("indexer", conf.Logfile, nil, conf.Loglevel, conf.LogFormat)
	defer lf.Close()

	ad, err := InitActionDispatcher(conf.Indexer, logger)
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
