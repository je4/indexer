package util

import (
	"emperror.dev/errors"
	"github.com/je4/indexer/v2/pkg/indexer"
	"github.com/je4/utils/v2/pkg/checksum"
	"github.com/je4/utils/v2/pkg/zLogger"
	"io"
	"io/fs"
)

type Indexer indexer.ActionDispatcher

func (idx *Indexer) Index(fsys fs.FS, path string, realname string, actions []string, digestAlgs []checksum.DigestAlgorithm, writer io.Writer, logger zLogger.ZWrapper) (*indexer.ResultV2, map[checksum.DigestAlgorithm]string, error) {
	if realname == "" {
		realname = path
	}
	ad := (*indexer.ActionDispatcher)(idx)
	fp, err := fsys.Open(path)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot open %s/%s", fsys, path)
	}

	idxRead, idxWrite := io.Pipe()
	csw, err := checksum.NewChecksumWriter(digestAlgs, io.MultiWriter(idxWrite, writer))
	if err != nil {
		idxWrite.Close()
		fp.Close()
		return nil, nil, errors.Wrapf(err, "cannot create ChecksumWriter for digests %v", digestAlgs)
	}

	go func() {
		defer func() {
			csw.Close()
			idxWrite.Close()
			fp.Close()
		}()
		_, err := io.Copy(csw, fp)
		if err != nil {
			// todo: channel with error
			logger.Errorf("cannot copy data: %v", err)
		}
	}()
	result, err := ad.Stream(idxRead, []string{realname}, actions)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot index '%s/%s'", fsys, path)
	}

	digests, err := csw.GetChecksums()
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot get checksums")
	}

	return result, digests, nil
}
