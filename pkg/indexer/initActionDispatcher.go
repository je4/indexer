package indexer

import (
	"emperror.dev/errors"
	"github.com/op/go-logging"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func stringMapToMimeRelevance(mimeRelevanceInterface map[string]ConfigMimeWeight) (map[int]MimeWeightString, error) {
	var mimeRelevance = map[int]MimeWeightString{}
	for keyStr, val := range mimeRelevanceInterface {
		key, err := strconv.Atoi(keyStr)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid key entry '%s' in 'MimeRelevance'", keyStr)
		}
		mimeRelevance[key] = MimeWeightString{
			Regexp: val.Regexp,
			Weight: val.Weight,
		}
	}
	return mimeRelevance, nil
}

var fsRegexp = regexp.MustCompile("^([^:]{2,}):(.+)$")

func InitActionDispatcher(fss map[string]fs.FS, conf IndexerConfig, logger *logging.Logger) (*ActionDispatcher, error) {
	mimeRelevance, err := stringMapToMimeRelevance(conf.MimeRelevance)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert config string map to mime relevance")
	}
	actionDispatcher := NewActionDispatcher(mimeRelevance)

	var signatureData []byte
	found := fsRegexp.FindStringSubmatch(conf.Siegfried.SignatureFile)
	if found == nil {
		signatureData, err = os.ReadFile(conf.Siegfried.SignatureFile)
		if err != nil {
			return nil, errors.Wrapf(err, "no siegfried signature file provided. using default signature file. please provide a recent signature file. %s", conf.Siegfried.SignatureFile)
			//			signatureData = datasiegfried.DefaultSig
		}
	} else {
		intFS, ok := fss[found[1]]
		if !ok {
			return nil, errors.Errorf("invalid filesystem %s", found[1])
		}
		signatureData, err = fs.ReadFile(intFS, strings.TrimLeft(found[2], "/"))
		if err != nil {
			return nil, errors.Wrapf(err, "no siegfried signature file provided. using default signature file. please provide a recent signature file. %s", conf.Siegfried.SignatureFile)
		}
	}
	_ = NewActionSiegfried("siegfried", signatureData, conf.Siegfried.MimeMap, nil, actionDispatcher)
	logger.Info("indexer action siegfried added")

	if conf.Checksum.Enabled {
		_ = NewActionChecksum(
			"checksum",
			conf.Checksum.Digest,
			nil,
			actionDispatcher,
		)
	}
	if conf.FFMPEG.Enabled {
		_ = NewActionFFProbe(
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
		_ = NewActionIdentifyV2("identify",
			conf.ImageMagick.Identify,
			conf.ImageMagick.Convert,
			conf.ImageMagick.Wsl,
			conf.ImageMagick.Timeout.Duration,
			conf.ImageMagick.Online, nil, actionDispatcher)
		logger.Info("indexer action identify added")
	}
	if conf.Tika.Enabled {
		_ = NewActionTika("tika",
			conf.Tika.AddressMeta,
			conf.Tika.Timeout.Duration,
			conf.Tika.RegexpMimeMeta,
			conf.Tika.RegexpMimeMetaNot,
			"",
			conf.Tika.Online,
			nil, actionDispatcher)
		logger.Info("indexer action tika added")

		_ = NewActionTika("fulltext",
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
