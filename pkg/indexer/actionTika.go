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
	"context"
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

// java -jar tika-server-1.24.jar -enableUnsecureFeatures -enableFileUrl --port=9997

type ActionTika struct {
	name       string
	url        string
	timeout    time.Duration
	regexpMime *regexp.Regexp
	caps       ActionCapability
	server     *Server
}

func NewActionTika(uri string, timeout time.Duration, regexpMime string, online bool, server *Server) Action {
	var caps ActionCapability = ACTFILEHEAD
	if online {
		caps |= ACTALLPROTO
	}
	at := &ActionTika{
		name:       "tika",
		url:        uri,
		timeout:    timeout,
		regexpMime: regexp.MustCompile(regexpMime),
		caps:       caps,
		server:     server,
	}
	server.AddAction(at)
	return at
}

func (at *ActionTika) GetCaps() ActionCapability {
	return ACTFILEHEAD
}

func (at *ActionTika) GetName() string {
	return at.name
}

func (at *ActionTika) Do(uri *url.URL, mimetype string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, []string, error) {
	if !at.regexpMime.MatchString(mimetype) {
		return nil, nil, ErrMimeNotApplicable
	}

	var dataOut io.Reader
	// local files need some adjustments...
	if uri.Scheme == "file" {
		filename, err := at.server.fm.Get(uri)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "invalid file uri %s", uri.String())
		}
		f, err := os.Open(filename)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot open: %s", filename)
		}
		defer f.Close()
		dataOut = f
	} else {
		//		filename = uri.String()
		resp, err := http.Get(uri.String())
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot load url: %s", uri.String())
		}
		defer resp.Body.Close()
		dataOut = resp.Body
	}

	client := &http.Client{}
	ctx, cancel := context.WithTimeout(context.Background(), at.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, at.url, dataOut)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot create tika request - %v", at.url)
	}
	req.Header.Add("Accept", "application/json")
	//req.Header.Add("fileUrl", uri.String())
	tresp, err := client.Do(req)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error in tika request - %v", at.url)
	}
	defer tresp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(tresp.Body)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error reading body - %v", at.url)
	}

	if tresp.StatusCode != http.StatusOK {
		return nil, nil, errors.New(fmt.Sprintf("status not ok - %v -> %v: %s", at.url, tresp.Status, string(bodyBytes)))
	}

	if bodyBytes[0] == '{' {
		bodyBytes = append([]byte{'['}, bodyBytes...)
		bodyBytes = append(bodyBytes, ']')
	}
	result := make([]map[string]interface{}, 0)
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error decoding json - %v", string(bodyBytes))
	}
	mimetypes := []string{}
	if len(result) > 0 {
		if mtype, ok := result[0]["Content-Type"]; ok {
			if mTypeString, ok := mtype.(string); ok {
				mimetypes = append(mimetypes, mTypeString)
			}
		}
	}
	return result, mimetypes, nil
}
