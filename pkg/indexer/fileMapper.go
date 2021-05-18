// Copyright 2021 Juergen Enge, info-age GmbH, Basel. All rights reserved.
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
	"errors"
	"fmt"
	"github.com/goph/emperror"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type FileMapper struct {
	mapping map[string]string
}

func NewFileMapper(mapping map[string]string) *FileMapper {
	return &FileMapper{mapping: mapping}
}

func (fm *FileMapper) Get(uri *url.URL) (string, error) {
	if uri.Scheme != "file" {
		return "", errors.New(fmt.Sprintf("cannot handle scheme %s: need file scheme", uri.Scheme))
	}
	var filename string
	var ok bool
	if uri.Host != "" {
		filename, ok = fm.mapping[strings.ToLower(uri.Host)]
		if !ok {
			return "", errors.New(fmt.Sprintf("no mapping for %s", uri.Host))
		}
	}
	p, err := url.QueryUnescape(uri.EscapedPath())
	if err != nil {
		return "", emperror.Wrapf(err, "cannot unescape %s", uri.EscapedPath())
	}
	filename = filepath.Join(filename, p)
	filename = filepath.Clean(filename)
	if runtime.GOOS == "windows" {
		filename = strings.TrimPrefix(filename, string(os.PathSeparator))
	}
	return filename, nil
}
