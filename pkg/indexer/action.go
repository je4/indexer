// Copyright 2020 Juergen Enge, info-age GmbH, Basel. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package indexer

import (
	"emperror.dev/errors"
	"fmt"
	"io"
	"net/url"
	"time"
)

type ActionCapability uint

const (
	ACTFILE   ActionCapability = 1 << iota // needs local file
	ACTHTTP                                // capable of HTTP
	ACTHTTPS                               // capable of HTTPS
	ACTHEAD                                // can deal with file head
	ACTSTREAM                              // can deal with stream

	ACTWEB      = ACTHTTPS | ACTHTTP
	ACTALLPROTO = ACTFILE | ACTHTTP | ACTHTTPS
	ACTALL      = ACTALLPROTO | ACTHEAD
	ACTFILEHEAD = ACTFILE | ACTHEAD
	ACTFILEFULL = ACTFILE & ^ACTHEAD
)

var ACTString map[ActionCapability]string = map[ActionCapability]string{
	ACTFILE:   "ACTFILE",
	ACTHTTP:   "ACTHTTP",
	ACTHTTPS:  "ACTHTTPS",
	ACTHEAD:   "ACTHEAD",
	ACTSTREAM: "ACTSTREAM",
}

var ACTAction map[string]ActionCapability = map[string]ActionCapability{
	"ACTFILE":   ACTFILE,
	"ACTHTTP":   ACTHTTP,
	"ACTHTTPS":  ACTHTTPS,
	"ACTHEAD":   ACTHEAD,
	"ACTSTREAM": ACTSTREAM,
}

// for toml decoding
func (a *ActionCapability) UnmarshalText(text []byte) error {
	var ok bool
	*a, ok = ACTAction[string(text)]
	if !ok {
		return fmt.Errorf("invalid actions capability: %s", string(text))
	}
	return nil
}

var ErrMimeNotApplicable = errors.New("mime type not applicable for actions")

type Action interface {
	Stream(contentType string, reader io.Reader, filename string) (*ResultV2, error)
	DoV2(filename string) (*ResultV2, error)
	CanHandle(contentType string, filename string) bool
	Do(uri *url.URL, contentType string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, []string, []string, error)
	GetName() string
	GetCaps() ActionCapability
	GetWeight() uint
}
