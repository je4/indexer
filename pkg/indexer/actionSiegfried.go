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
	"github.com/goph/emperror"
	"github.com/richardlehane/siegfried"
	"github.com/richardlehane/siegfried/pkg/pronom"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type ActionSiegfried struct {
	name    string
	sf      *siegfried.Siegfried
	mimeMap map[string]string
	server  *Server
}

func NewActionSiegfried(signatureFile string, mimeMap map[string]string, server *Server) Action {
	sf, err := siegfried.Load(signatureFile)
	if err != nil {
		log.Fatalln(err)
	}
	as := &ActionSiegfried{name: "siegfried", sf: sf, mimeMap: mimeMap, server: server}
	server.AddAction(as)
	return as
}

func (as *ActionSiegfried) GetCaps() ActionCapability {
	return ACTFILEHEAD
}

func (as *ActionSiegfried) GetName() string {
	return as.name
}

func (as *ActionSiegfried) Do(uri *url.URL, mimetype *string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, error) {
	filename, err := as.server.fm.Get(uri)
	if err != nil {
		return nil, emperror.Wrapf(err, "no file url")
	}

	fp, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot open file %s", filename)
	}
	defer fp.Close()

	ident, err := as.sf.Identify(fp, filepath.Base(filename), "")
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot identify file %s", filename)
	}
	for _, id := range ident {
		if pid, ok := id.(pronom.Identification); ok {
			rel1 := as.server.MimeRelevance(*mimetype)
			rel2 := as.server.MimeRelevance(pid.MIME)
			if rel2 > rel1 {
				*mimetype = pid.MIME
			}
			if mime, ok := as.mimeMap[pid.ID]; ok {
				rel1 := as.server.MimeRelevance(*mimetype)
				rel2 := as.server.MimeRelevance(mime)
				if rel2 > rel1 {
					*mimetype = mime
				}

			}

		}
	}
	return ident, nil
}
