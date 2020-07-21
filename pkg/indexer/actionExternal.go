// Copyright 2020 Juergen Enge, info-age GmbH, Basel. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Start-Process -FilePath c:/daten/go/bin/sf.exe -Args "-serve localhost:5138" -Wait -NoNewWindow
// c:/daten/go/bin/sf.exe -serve localhost:5138

package indexer

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/goph/emperror"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type ExternalActionCalltype uint

const (
	EACTURL      ExternalActionCalltype = 1 << iota // url with placehoder for full path
	EACTJSONPOST                                    // send json struct via post
)

var EACTString map[ExternalActionCalltype]string = map[ExternalActionCalltype]string{
	EACTURL:      "EACTURL",
	EACTJSONPOST: "EACTJSONPOST",
}

var EACTAction map[string]ExternalActionCalltype = map[string]ExternalActionCalltype{
	"EACTURL":      EACTURL,
	"EACTJSONPOST": EACTJSONPOST,
}

// for toml decoding
func (a *ExternalActionCalltype) UnmarshalText(text []byte) error {
	var ok bool
	*a, ok = EACTAction[string(text)]
	if !ok {
		return fmt.Errorf("invalid action capability: %s", string(text))
	}
	return nil
}

type ActionExternal struct {
	name       string
	url        string
	capability ActionCapability
	callType   ExternalActionCalltype
	server     *Server
	mimetype   *regexp.Regexp
}

func NewActionExternal(
	name, address string,
	capability ActionCapability,
	callType ExternalActionCalltype,
	mimetype string,
	server *Server) Action {
	ae := &ActionExternal{
		name:       name,
		url:        address,
		capability: capability,
		callType:   callType,
		mimetype:   regexp.MustCompile(mimetype),
		server:     server,
	}
	server.AddAction(ae)
	return ae
}

func (as *ActionExternal) GetCaps() ActionCapability {
	return as.capability
}

func (as *ActionExternal) GetName() string {
	return as.name
}

func (as *ActionExternal) Do(
	uri *url.URL,
	mimetype *string,
	width *uint, height *uint,
	duration *time.Duration) (interface{}, error) {
	switch uri.Scheme {
	case "file":
		if as.capability&ACTFILE != ACTFILE {
			return nil, fmt.Errorf("invalid capability for file url scheme")
		}
	case "http":
		if as.capability&ACTHTTP != ACTHTTP {
			return nil, fmt.Errorf("invalid capability for http url scheme")
		}
	case "https":
		if as.capability&ACTHTTPS != ACTHTTPS {
			return nil, fmt.Errorf("invalid capability for https url scheme")
		}
	}

	if !as.mimetype.MatchString(*mimetype) {
		return nil, ErrMimeNotApplicable
	}

	var resp *http.Response
	if as.callType == EACTURL {
		filename, err := as.server.fm.Get(uri)
		if err != nil {
			return nil, emperror.Wrapf(err, "no file url")
		}
		urlstring := strings.Replace(as.url, "[[PATH]]", strings.Replace(url.QueryEscape(filepath.ToSlash(filename)), "+", "%20", -1), -1)

		resp, err = http.Get(urlstring)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot query %v - %v", as.name, urlstring)
		}
	} else if as.callType == EACTJSONPOST {
		return nil, fmt.Errorf("JSONPOST CallType not implemented")
	} else {
		return nil, fmt.Errorf("unknown calltype")
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, emperror.Wrapf(err, "error reading body")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("status not ok - %v: %s", resp.Status, string(bodyBytes)))
	}

	var result interface{}
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, emperror.Wrapf(err, "error decoding json - %v", string(bodyBytes))
	}
	return result, nil
}
