package util

import (
	"emperror.dev/errors"
	"github.com/je4/indexer/v2/pkg/indexer"
	"github.com/op/go-logging"
	"os"
	"strconv"
)

// InitIndexer
// initializes an ActionDispatcher with Siegfried, ImageMagick, FFPMEG and Tika
// the actions are named "siegfried", "identify", "ffprobe", "tika" and "fulltext"
func InitIndexer(conf *Config, logger *logging.Logger) (ad *Indexer, err error) {
	var relevance = map[int]indexer.MimeWeightString{}
	if conf.MimeRelevance != nil {
		for key, val := range conf.MimeRelevance {
			num, err := strconv.Atoi(key)
			if err != nil {
				logger.Errorf("cannot convert mimerelevance key '%s' to int", key)
				continue
			}
			relevance[num] = indexer.MimeWeightString(val)
		}
	}

	ad = (*Indexer)(indexer.NewActionDispatcher(relevance))
	var signature []byte
	if conf.Siegfried.SignatureFile == "" {
		signature = siegfriedSignatures
	} else {
		signature, err = os.ReadFile(conf.Siegfried.SignatureFile)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read siegfried signature file '%s'", conf.Siegfried.SignatureFile)
		}
	}
	_ = indexer.NewActionSiegfried("siegfried", signature, conf.Siegfried.MimeMap, nil, (*indexer.ActionDispatcher)(ad))
	logger.Info("indexer action siegfried added")

	if conf.FFMPEG != nil && conf.FFMPEG.Enabled {
		_ = indexer.NewActionFFProbe(
			"ffprobe",
			conf.FFMPEG.FFProbe,
			conf.FFMPEG.Wsl,
			conf.FFMPEG.Timeout.Duration,
			conf.FFMPEG.Online,
			conf.FFMPEG.Mime,
			nil,
			(*indexer.ActionDispatcher)(ad))
		logger.Info("indexer action ffprobe added")
	}
	if conf.ImageMagick != nil && conf.ImageMagick.Enabled {
		_ = indexer.NewActionIdentifyV2("identify",
			conf.ImageMagick.Identify,
			conf.ImageMagick.Convert,
			conf.ImageMagick.Wsl,
			conf.ImageMagick.Timeout.Duration,
			conf.ImageMagick.Online,
			nil,
			(*indexer.ActionDispatcher)(ad))
		logger.Info("indexer action identify added")
	}
	if conf.Tika != nil && conf.Tika.Enabled {
		if conf.Tika.AddressMeta != "" {
			_ = indexer.NewActionTika("tika",
				conf.Tika.AddressMeta,
				conf.Tika.Timeout.Duration,
				conf.Tika.RegexpMimeMeta,
				conf.Tika.RegexpMimeNotMeta,
				"",
				conf.Tika.Online, nil,
				(*indexer.ActionDispatcher)(ad))
			logger.Info("indexer action tika added")
		}

		if conf.Tika.AddressFulltext != "" {
			_ = indexer.NewActionTika("fulltext",
				conf.Tika.AddressFulltext,
				conf.Tika.Timeout.Duration,
				conf.Tika.RegexpMimeFulltext,
				conf.Tika.RegexpMimeNotFulltext,
				"X-TIKA:content",
				conf.Tika.Online,
				nil,
				(*indexer.ActionDispatcher)(ad))
			logger.Info("indexer action fulltext added")
		}

	}

	return
}
