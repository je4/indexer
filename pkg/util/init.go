package util

import (
	"emperror.dev/errors"
	"github.com/je4/indexer/v3/pkg/indexer"
	"github.com/je4/utils/v2/pkg/zLogger"
	"os"
	"strconv"
)

// InitIndexer
// initializes an ActionDispatcher with Siegfried, ImageMagick, FFPMEG and Tika
// the actions are named "siegfried", "identify", "ffprobe", "tika" and "fulltext"
func InitIndexer(conf *indexer.IndexerConfig, logger zLogger.ZLogger) (ad *Indexer, err error) {
	var relevance = map[int]indexer.MimeWeightString{}
	if conf.MimeRelevance != nil {
		for key, val := range conf.MimeRelevance {
			num, err := strconv.Atoi(key)
			if err != nil {
				logger.Error().Msgf("cannot convert mimerelevance key '%s' to int", key)
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
	_ = indexer.NewActionSiegfried("siegfried", signature, conf.Siegfried.MimeMap, conf.Siegfried.TypeMap, nil, (*indexer.ActionDispatcher)(ad))
	logger.Info().Msg("indexer action siegfried added")

	if conf.XML.Enabled {
		_ = indexer.NewActionXML("xml", conf.XML.Format, nil, (*indexer.ActionDispatcher)(ad))
		logger.Info().Msg("indexer action xml added")
	}

	if conf.FFMPEG.Enabled {
		_ = indexer.NewActionFFProbe(
			"ffprobe",
			conf.FFMPEG.FFProbe,
			conf.FFMPEG.Wsl,
			conf.FFMPEG.Timeout.Duration,
			conf.FFMPEG.Online,
			conf.FFMPEG.Mime,
			nil,
			(*indexer.ActionDispatcher)(ad))
		logger.Info().Msg("indexer action ffprobe added")
	}
	if conf.ImageMagick.Enabled {
		_ = indexer.NewActionIdentifyV2("identify",
			conf.ImageMagick.Identify,
			conf.ImageMagick.Convert,
			conf.ImageMagick.Wsl,
			conf.ImageMagick.Timeout.Duration,
			conf.ImageMagick.Online,
			nil,
			(*indexer.ActionDispatcher)(ad))
		logger.Info().Msg("indexer action identify added")
	}
	if conf.Tika.Enabled {
		if conf.Tika.AddressMeta != "" {
			_ = indexer.NewActionTika("tika",
				conf.Tika.AddressMeta,
				conf.Tika.Timeout.Duration,
				conf.Tika.RegexpMimeMeta,
				conf.Tika.RegexpMimeMetaNot,
				"",
				conf.Tika.Online, nil,
				(*indexer.ActionDispatcher)(ad))
			logger.Info().Msg("indexer action tika added")
		}

		if conf.Tika.AddressFulltext != "" {
			_ = indexer.NewActionTika("fulltext",
				conf.Tika.AddressFulltext,
				conf.Tika.Timeout.Duration,
				conf.Tika.RegexpMimeFulltext,
				conf.Tika.RegexpMimeFulltextNot,
				"X-TIKA:content",
				conf.Tika.Online,
				nil,
				(*indexer.ActionDispatcher)(ad))
			logger.Info().Msg("indexer action fulltext added")
		}

	}

	return
}
