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
	"golang.org/x/exp/slices"
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

type ActionIdentifyV2 struct {
	name     string
	identify string
	convert  string
	wsl      bool
	timeout  time.Duration
	caps     ActionCapability
	server   *Server
	mimeMap  map[string]string
}

func NewActionIdentifyV2(identify, convert string, wsl bool, timeout time.Duration, online bool, server *Server) Action {
	var caps ActionCapability = ACTFILEHEAD
	if online {
		caps |= ACTALLPROTO
	}
	ai := &ActionIdentifyV2{
		name:     "identify2",
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
						t := m.Type[6:]
						if t != "" && !strings.ContainsAny(t, ".-") {
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

func (ai *ActionIdentifyV2) GetWeight() uint {
	return 50
}

func (ai *ActionIdentifyV2) GetCaps() ActionCapability {
	return ACTFILEHEAD
}

func (ai *ActionIdentifyV2) GetName() string {
	return ai.name
}

func (ai *ActionIdentifyV2) Do(uri *url.URL, mimetype string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, []string, []string, error) {
	var metadata = FullMagickResult{
		Frames: []*Geometry{},
	}
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

	var meta = []*MagickResult{}
	if err = json.Unmarshal([]byte(out.String()), &meta); err != nil {
		return nil, nil, nil, errors.Wrapf(err, "cannot unmarshall metadata: %s", out.String())
	}
	if len(meta) == 0 {
		return nil, nil, nil, errors.New("no metadata from imagemagick found")
	}
	metadata.Magick = meta[0]
	if metadata.Magick.Image != nil {
		metadata.Magick.Image.Name = uri.String()
	}
	mimetypes := []string{}
	for _, m := range meta {
		if m.Image == nil {
			continue
		}
		if m.Image.MimeType != "" {
			mimetypes = append(mimetypes, m.Image.MimeType)
		}
		if m.Image.Geometry != nil {
			metadata.Frames = append(metadata.Frames, m.Image.Geometry)
			if uint(m.Image.Geometry.Width) > *width {
				*width = uint(m.Image.Geometry.Width)
			}
			if uint(m.Image.Geometry.Height) > *height {
				*height = uint(m.Image.Geometry.Height)
			}
		}
	}
	slices.Sort(mimetypes)
	mimetypes = slices.Compact(mimetypes)
	return metadata, mimetypes, nil, nil
}

var (
	_ Action = (*ActionIdentifyV2)(nil)
)