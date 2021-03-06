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
package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/goph/emperror"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
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
	}
	server.AddAction(ai)
	return ai
}

func (ai *ActionIdentify) GetCaps() ActionCapability {
	return ACTFILEHEAD
}

func (ai *ActionIdentify) GetName() string {
	return ai.name
}

func (ai *ActionIdentify) Do(uri *url.URL, mimetype *string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, error) {
	var metadata = make(map[string]interface{})
	var metadataInt interface{}
	//	var metadatalist = []map[string]interface{}{}
	var filename string
	var err error

	if !regexIdentifyMime.MatchString(*mimetype) {
		return nil, ErrMimeNotApplicable
	}

	var dataOut io.Reader
	// local files need some adjustments...
	if uri.Scheme == "file" {
		filename, err = ai.server.fm.Get(uri)
		if err != nil {
			return nil, emperror.Wrapf(err, "invalid file uri %s", uri.String())
		}
		f, err := os.Open(filename)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot open: %s", filename)
		}
		defer f.Close()
		dataOut = f
	} else {
		//		filename = uri.String()
		resp, err := http.Get(uri.String())
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot load url: %s", uri.String())
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil, errors.New(fmt.Sprintf("invalid status %v - %v for %s", resp.StatusCode, resp.StatusCode, uri.String()))
		}
		dataOut = resp.Body
	}

	cmdparam := []string{"-", "json:-"}
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
		return nil, emperror.Wrapf(err, "error executing (%s %s): %v", cmdfile, cmdparam, out.String())
	}

	if err = json.Unmarshal([]byte(out.String()), &metadataInt); err != nil {
		return nil, emperror.Wrapf(err, "cannot unmarshall metadata: %s", out.String())
	}

	switch val := metadataInt.(type) {
	case []interface{}:
		// todo: check for content and type
		if len(val) != 1 {
			return nil, fmt.Errorf("wrong number of objects in image magick result list - %v", len(val))
		}
		var ok bool
		metadata, ok = val[0].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("wrong object type in image magick result - %T", val[0])
		}
	case map[string]interface{}:
		metadata = val
	default:
		return nil, fmt.Errorf("invalid return type from image magick - %T", val)
	}

	_image, ok := metadata["image"]
	if !ok {
		return nil, emperror.Wrapf(err, "no image field in %s", out.String())
	}
	// calculate mimetype and dimensions
	image, ok := _image.(map[string]interface{})
	_mimetype, ok := image["mimeType"].(string)
	if ok {
		if ai.server.MimeRelevance(_mimetype) > ai.server.MimeRelevance(*mimetype) {
			*mimetype = _mimetype
		}
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

	return metadata, nil
}
