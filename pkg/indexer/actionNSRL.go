package indexer

import (
	"github.com/dgraph-io/badger"
	"net/url"
	"time"
)

type ActionNSRL struct {
	name   string
	caps   ActionCapability
	server *Server
	nsrldb *badger.DB
}

func NewActionNSRL(nsrldb *badger.DB, server *Server) Action {
	an := &ActionNSRL{name: "NSRL", nsrldb: nsrldb, server: server, caps: ACTALL}
	server.AddAction(an)
	return an
}

func (aNSRL *ActionNSRL) GetCaps() ActionCapability {
	return aNSRL.caps
}

func (aNSRL *ActionNSRL) GetName() string {
	return aNSRL.name
}

func (aNSRL *ActionNSRL) Do(uri *url.URL, mimetype *string, width *uint, height *uint, duration *time.Duration) (interface{}, error) {
	var metadata interface{}
	return metadata, nil
}
