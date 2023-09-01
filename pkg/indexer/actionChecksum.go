package indexer

import (
	"emperror.dev/errors"
	"github.com/je4/utils/v2/pkg/checksum"
	"io"
	"net/url"
	"os"
	"time"
)

// Copyright 2021 Juergen Enge, info-age GmbH, Basel. All rights reserved.
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

type ActionChecksum struct {
	name    string
	server  *Server
	digests []checksum.DigestAlgorithm
}

func (as *ActionChecksum) CanHandle(contentType string, filename string) bool {
	return true
}

func NewActionChecksum(name string, digests []checksum.DigestAlgorithm, server *Server, ad *ActionDispatcher) Action {
	as := &ActionChecksum{name: name, server: server, digests: digests}
	ad.RegisterAction(as)
	return as
}

func (as *ActionChecksum) GetWeight() uint {
	return 10
}

func (as *ActionChecksum) GetCaps() ActionCapability {
	return ACTSTREAM
}

func (as *ActionChecksum) GetName() string {
	return as.name
}

func (as *ActionChecksum) Stream(contentType string, reader io.Reader, filename string) (*ResultV2, error) {
	cw, err := checksum.NewChecksumWriter(as.digests)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create checksum writer")
	}
	if _, err := io.Copy(cw, reader); err != nil {
		return nil, errors.Wrap(err, "cannot copy stream data")
	}
	cw.Close()
	checksums, err := cw.GetChecksums()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get checksums")
	}
	var result = NewResultV2()
	result.Metadata[as.GetName()] = checksums
	for digest, val := range checksums {
		result.Checksum[string(digest)] = val
	}
	return result, nil
}

func (as *ActionChecksum) DoV2(filename string) (*ResultV2, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open file '%s'", filename)
	}
	defer reader.Close()
	cw, err := checksum.NewChecksumWriter(as.digests)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create checksum writer")
	}
	if _, err := io.Copy(cw, reader); err != nil {
		return nil, errors.Wrap(err, "cannot copy stream data")
	}
	cw.Close()
	checksums, err := cw.GetChecksums()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get checksums")
	}
	var result = NewResultV2()
	result.Metadata[as.GetName()] = checksums
	return result, nil
}

func (as *ActionChecksum) Do(uri *url.URL, contentType string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, []string, []string, error) {
	filename, err := as.server.fm.Get(uri)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "no file url")
	}

	fp, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "cannot open file %s", filename)
	}
	defer fp.Close()

	result, err := as.Stream("", fp, filename)
	if err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	return result.Metadata[as.GetName()], result.Mimetypes, result.Pronoms, nil
}

var (
	_ Action = &ActionChecksum{}
)
