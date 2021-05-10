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
	"strings"
	"time"
)

type SFIdentifier struct {
	Name    string `json:"name,omitempty"`
	Details string `json:"details,omitempty"`
}

type SFMatches struct {
	Ns      string `json:"ns,omitempty"`
	Id      string `json:"id,omitempty"`
	Format  string `json:"format,omitempty"`
	Version string `json:"version,omitempty"`
	Mime    string `json:"mime,omitempty"`
	Basis   string `json:"basis,omitempty"`
	Warning string `json:"warning,omitempty"`
}

type SFFiles struct {
	Filename string      `json:"filename,omitempty"`
	Filesize int64       `json:"filesize,omitempty"`
	Modified string      `json:"modified,omitempty"`
	Errors   string      `json:"errors,omitempty"`
	Matches  []SFMatches `json:"matches,omitempty"`
}

type SF struct {
	Siegfried   string         `json:"siegfried,omitempty"`
	Scandate    string         `json:"scandate,omitempty"`
	Signature   string         `json:"signature,omitempty"`
	Created     string         `json:"created,omitempty"`
	Identifiers []SFIdentifier `json:"identfiers,omitempty"`
	Files       []SFFiles      `json:"files,omitempty"`
}

type ActionSiegfried struct {
	name   string
	url    string
	server *Server
}

func NewActionSiegfried(uri string, server *Server) Action {
	as := &ActionSiegfried{name: "siegfried", url: uri, server: server}
	server.AddAction(as)
	return as
}

func (as *ActionSiegfried) GetCaps() ActionCapability {
	return ACTFILEHEAD
}

func (as *ActionSiegfried) GetName() string {
	return as.name
}

func (as *ActionSiegfried) Do(uri *url.URL, mimetype *string, width *uint, height *uint, duration *time.Duration) (interface{}, error) {
	filename, err := as.server.fm.Get(uri)
	if err != nil {
		return nil, emperror.Wrapf(err, "no file url")
	}
	urlstring := strings.Replace(as.url, "[[PATH]]", strings.Replace(url.PathEscape(filepath.ToSlash(filename)), "+", "%20", -1), -1)

	resp, err := http.Get(urlstring)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot query siegfried - %v", urlstring)
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, emperror.Wrapf(err, "error reading body - %v", urlstring)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("status not ok - %v -> %v: %s", urlstring, resp.Status, string(bodyBytes)))
	}

	result := SF{}
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, emperror.Wrapf(err, "error decoding json - %v", string(bodyBytes))
	}
	if len(result.Files) == 0 {
		return nil, emperror.Wrapf(err, "no file in sf result - %v", string(bodyBytes))
	}

	// change mimetype if we have the better one
	mimeRelevance := MimeRelevance(*mimetype)
	for key, m := range result.Files[0].Matches {
		mr := MimeRelevance(m.Mime)
		if mr > mimeRelevance {
			*mimetype = m.Mime
			mimeRelevance = mr
		}
		if m.Warning == "extension mismatch" {
			result.Files[0].Matches[key].Warning = ""
		}
	}
	return result.Files[0].Matches, nil
}
