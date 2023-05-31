// Package util
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
package util

import (
	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/indexer/v2/pkg/indexer"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

type duration struct {
	Duration time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type ConfigClamAV struct {
	Enabled  bool
	Timeout  duration
	ClamScan string
	Wsl      bool
}

type ConfigSiegfried struct {
	//Address string
	Enabled       bool
	SignatureFile string
	MimeMap       map[string]string
}

type ConfigTika struct {
	Timeout               duration
	AddressMeta           string
	RegexpMimeMeta        string
	RegexpMimeNotMeta     string
	AddressFulltext       string
	RegexpMimeFulltext    string
	RegexpMimeNotFulltext string
	Online                bool
	Enabled               bool
}

type ConfigFFMPEG struct {
	FFProbe string
	Wsl     bool
	Timeout duration
	Online  bool
	Enabled bool
	Mime    []indexer.FFMPEGMime
}

type ConfigImageMagick struct {
	Identify string
	Convert  string
	Wsl      bool
	Timeout  duration
	Online   bool
	Enabled  bool
}

type ExternalAction struct {
	Name,
	Address,
	Mimetype string
	ActionCapabilities []indexer.ActionCapability
	CallType           indexer.ExternalActionCalltype
}

type FileMap struct {
	Alias  string
	Folder string
}
type ConfigNSRL struct {
	Enabled bool
	Badger  string
}

type MimeWeight struct {
	Regexp string
	Weight int
}

type Config struct {
	Siegfried     *ConfigSiegfried
	FFMPEG        *ConfigFFMPEG
	ImageMagick   *ConfigImageMagick
	Tika          *ConfigTika
	External      []ExternalAction
	FileMap       []FileMap
	URLRegexp     []string
	NSRL          *ConfigNSRL
	Clamav        *ConfigClamAV
	MimeRelevance map[string]MimeWeight
}

func LoadConfig(tomlBytes []byte) (*Config, error) {
	type confStruct struct {
		Indexer *Config
	}
	var conf = &confStruct{}

	if err := toml.Unmarshal(tomlBytes, conf); err != nil {
		return nil, errors.Wrapf(err, "Error unmarshalling config")
	}
	if conf.Indexer.Siegfried.SignatureFile == "" {
		user, err := user.Current()
		if err != nil {
			return nil, errors.Wrap(err, "cannot get current user")
		}
		fp := filepath.Join(user.HomeDir, "siegfried", "default.sig")
		fi, err := os.Stat(fp)
		if err == nil && !fi.IsDir() {
			conf.Indexer.Siegfried.SignatureFile = fp
		}
	}
	return conf.Indexer, nil
}
