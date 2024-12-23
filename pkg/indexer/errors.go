package indexer

import (
	"github.com/je4/indexer/v3/internal"
	"github.com/je4/utils/v2/pkg/zLogger"
	archiveerror "github.com/ocfl-archive/error/pkg/error"
)

var ErrorFactory = archiveerror.NewFactory("INDEXER")

type errorID = archiveerror.ID

const (
	ErrorIndexerInit = "ErrorIndexerInit"
)

func configErrorFactory(logger zLogger.ZLogger) {
	var err error
	const errorsEmbedToml string = "errors.toml"
	archiveErrs, err := archiveerror.LoadTOMLFileFS(internal.InternalFS, errorsEmbedToml)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot load error config file")
	}

	if err := ErrorFactory.RegisterErrors(archiveErrs); err != nil {
		logger.Fatal().Err(err).Msg("cannot load error config file")
	}
}
