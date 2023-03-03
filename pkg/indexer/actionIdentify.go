// Package indexer
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
	"bytes"
	"context"
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var regexIdentifyMime = regexp.MustCompile("^image/")

type ActionIdentify struct {
	name     string
	identify string
	convert  string
	wsl      bool
	timeout  time.Duration
	caps     ActionCapability
	server   *Server
	mimeMap  map[string]string
}

func NewActionIdentify(identify, convert string, wsl bool, timeout time.Duration, online bool, server *Server) Action {
	var caps ActionCapability = ACTFILEHEAD
	if online {
		caps |= ACTALLPROTO
	}
	ai := &ActionIdentify{
		name:     "identify",
		identify: identify,
		convert:  convert,
		wsl:      wsl,
		timeout:  timeout,
		caps:     caps,
		server:   server,
		mimeMap:  map[string]string{},
	}
	if mime, err := GetMagickMime(); err == nil {
		if mime != nil {
			for _, m := range mime {
				if m.Acronym != nil && *m.Acronym != "" {
					ai.mimeMap[m.Type] = *m.Acronym
				} else {
					m.Type = strings.ToLower(m.Type)
					if strings.HasPrefix(m.Type, "image/") {
						t := strings.TrimPrefix(m.Type, "image/")
						if t != "" {
							ai.mimeMap[m.Type] = t
						}
					}
				}
			}
		}
	}
	server.AddAction(ai)
	return ai
}

func (ai *ActionIdentify) GetWeight() uint {
	return 50
}

func (ai *ActionIdentify) GetCaps() ActionCapability {
	return ACTFILEHEAD
}

func (ai *ActionIdentify) GetName() string {
	return ai.name
}

func (ai *ActionIdentify) Do(uri *url.URL, mimetype string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, []string, []string, error) {
	var metadata = make(map[string]interface{})
	var metadataInt interface{}
	//	var metadatalist = []map[string]interface{}{}
	var filename string
	var err error

	if !regexIdentifyMime.MatchString(mimetype) {
		return nil, nil, nil, ErrMimeNotApplicable
	}

	var dataOut io.Reader
	// local files need some adjustments...
	if uri.Scheme == "file" {
		filename, err = ai.server.fm.Get(uri)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "invalid file uri %s", uri.String())
		}
		f, err := os.Open(filename)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "cannot open: %s", filename)
		}
		defer f.Close()
		dataOut = f
	} else {
		//		filename = uri.String()
		resp, err := http.Get(uri.String())
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "cannot load url: %s", uri.String())
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil, nil, nil, errors.New(fmt.Sprintf("invalid status %v - %v for %s", resp.StatusCode, resp.StatusCode, uri.String()))
		}
		dataOut = resp.Body
	}

	infile := "-"
	if t, ok := ai.mimeMap[mimetype]; ok {
		infile = t + ":-"
	} else {
		t := strings.TrimPrefix(mimetype, "image/")
		if len(t) > 0 {
			infile = t + ":-"
		}
	}
	cmdparam := []string{infile, "json:-"}
	cmdfile := ai.convert
	if ai.wsl {
		cmdparam = append([]string{cmdfile}, cmdparam...)
		cmdfile = "wsl"
	}

	var out bytes.Buffer
	out.Grow(1024 * 1024) // 1MB size
	ctx, cancel := context.WithTimeout(context.Background(), ai.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdfile, cmdparam...)
	cmd.Stdin = dataOut
	cmd.Stdout = &out

	err = cmd.Run()
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "error executing (%s %s) for file '%s': %v", cmdfile, cmdparam, filename, out.String())
	}

	if err = json.Unmarshal([]byte(out.String()), &metadataInt); err != nil {
		return nil, nil, nil, errors.Wrapf(err, "cannot unmarshall metadata: %s", out.String())
	}

	switch val := metadataInt.(type) {
	case []interface{}:
		// todo: check for content and type
		if len(val) > 0 {
			metadata = val[0].(map[string]interface{})
		} else {
			return nil, nil, nil, errors.New("empty image magick result list")
		}
		/*
			if len(val) != 1 {
				return nil, nil, nil, fmt.Errorf("wrong number of objects in image magick result list - %v", len(val))
			}
			var ok bool
			metadata, ok = val[0].(map[string]interface{})
			if !ok {
				return nil, nil, nil, fmt.Errorf("wrong object type in image magick result - %T", val[0])
			}
		*/
	case map[string]interface{}:
		metadata = val
	default:
		return nil, nil, nil, fmt.Errorf("invalid return type from image magick - %T", val)
	}

	_image, ok := metadata["image"]
	if !ok {
		return nil, nil, nil, errors.Wrapf(err, "no image field in %s", out.String())
	}
	// calculate mimetype and dimensions
	image, ok := _image.(map[string]interface{})
	_mimetype, ok := image["mimeType"].(string)
	mimetypes := []string{}
	if ok {
		mimetypes = append(mimetypes, _mimetype)
	}
	_geometry, ok := image["geometry"].(map[string]interface{})
	if ok {
		w, ok := _geometry["width"].(float64)
		if ok {
			*width = uint(w)
		}
		h, ok := _geometry["height"].(float64)
		if ok {
			*height = uint(h)
		}
	}

	return metadata, mimetypes, nil, nil
}

var (
	_ Action = (*ActionIdentify)(nil)
)
