package indexer

import (
	"errors"
	"net/url"
	"time"
)

type ActionCapability uint

const (
	ACTFILE  ActionCapability = 1 << iota // needs local file
	ACTHTTP                               // capable of HTTP
	ACTHTTPS                              // capable of HTTPS
	ACTHEAD                               // can deal with file head

	ACTWEB      = ACTHTTPS | ACTHTTP
	ACTALLPROTO = ACTFILE | ACTHTTP | ACTHTTPS
	ACTALL      = ACTALLPROTO | ACTHEAD
	ACTFILEHEAD = ACTFILE | ACTHEAD
)

var ErrMimeNotApplicable = errors.New("mime type not applicable for action")

type Action interface {
	Do(uri *url.URL, mimetype *string, width *uint, height *uint, duration *time.Duration) (interface{}, error)
	GetName() string
	GetCaps() ActionCapability
}
