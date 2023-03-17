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
	"bufio"
	"bytes"
	"context"
	"emperror.dev/errors"
	"io"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

type ActionClamAV struct {
	name    string
	clamav  string
	wsl     bool
	timeout time.Duration
	caps    ActionCapability
	server  *Server
}

func (ac *ActionClamAV) CanHandle(contentType string, filename string) bool {
	return true
}

func (ac *ActionClamAV) Stream(contentType string, reader io.Reader, filename string) (*ResultV2, error) {
	return nil, errors.New("clamav does not support streaming")
}

func NewActionClamAV(clamav string, wsl bool, timeout time.Duration, server *Server, ad *ActionDispatcher) Action {
	var caps = ACTFILEFULL
	ac := &ActionClamAV{name: "clamav", clamav: clamav, wsl: wsl, timeout: timeout, caps: caps, server: server}
	ad.RegisterAction(ac)
	return ac
}

func (ac *ActionClamAV) GetWeight() uint {
	return 100
}

func (ac *ActionClamAV) GetCaps() ActionCapability {
	return ac.caps
}

func (ac *ActionClamAV) GetName() string {
	return ac.name
}

func (ac *ActionClamAV) Do(uri *url.URL, contentType string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, []string, []string, error) {
	var filename string
	var err error

	// local files need some adjustments...
	if uri.Scheme == "file" {
		filename, err = ac.server.fm.Get(uri)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "invalid file uri %s", uri.String())
		}
		if ac.wsl {
			filename = pathToWSL(filename)
		}
	} else {
		filename = uri.String()
	}

	cmdparam := []string{"--no-summary", filename}
	cmdfile := ac.clamav
	if ac.wsl {
		cmdparam = append([]string{cmdfile}, cmdparam...)
		cmdfile = "wsl"
	}

	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), ac.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cmdfile, cmdparam...)
	cmd.Stdout = &out

	err = cmd.Run()
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "error executing (%s %s): %v", cmdfile, cmdparam, out.String())
	}

	result := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(out.String()))
	for sc.Scan() {
		line := sc.Text()
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return result, nil, nil, nil
}

var (
	_ Action = &ActionClamAV{}
)
