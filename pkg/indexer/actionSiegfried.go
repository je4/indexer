package indexer

import (
	"github.com/goph/emperror"
	"github.com/richardlehane/siegfried"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type ActionSiegfried struct {
	name   string
	sf     *siegfried.Siegfried
	server *Server
}

func NewActionSiegfried(signatureFile string, server *Server) Action {
	sf, err := siegfried.Load(signatureFile)
	if err != nil {
		log.Fatalln(err)
	}
	as := &ActionSiegfried{name: "siegfried", sf: sf, server: server}
	server.AddAction(as)
	return as
}

func (as *ActionSiegfried) GetCaps() ActionCapability {
	return ACTFILEHEAD
}

func (as *ActionSiegfried) GetName() string {
	return as.name
}

func (as *ActionSiegfried) Do(uri *url.URL, mimetype *string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, error) {
	filename, err := as.server.fm.Get(uri)
	if err != nil {
		return nil, emperror.Wrapf(err, "no file url")
	}

	fp, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot open file %s", filename)
	}
	defer fp.Close()

	ident, err := as.sf.Identify(fp, filepath.Base(filename), "")
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot identify file %s", filename)
	}
	return ident, nil
}
