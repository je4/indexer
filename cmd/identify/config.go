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
package main

import (
	"github.com/BurntSushi/toml"
	"github.com/je4/indexer/pkg/indexer"
	"log"
	"os"
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

type ConfigSiegfried struct {
	Address string
	Enabled bool
}

type ConfigTika struct {
	Address    string
	Timeout    duration
	RegexpMime string
	Online     bool
	Enabled    bool
}

type ConfigFFMPEG struct {
	FFProbe string
	Wsl     bool
	Timeout duration
	Online  bool
	Enabled bool
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

type SFTP struct {
	Knownhosts string
	Password   string
	PrivateKey []string
}

type Config struct {
	ErrorTemplate   string
	Logfile         string
	Loglevel        string
	AccessLog       string
	CertPEM         string
	KeyPEM          string
	Addr            string
	JwtKey          string
	InsecureCert    bool
	JwtAlg          []string
	TempDir         string
	HeaderTimeout   duration
	HeaderSize      int64
	DownloadMime    string `toml:"forcedownload"`
	MaxDownloadSize int64
	Siegfried       ConfigSiegfried
	FFMPEG          ConfigFFMPEG
	ImageMagick     ConfigImageMagick
	Tika            ConfigTika
	External        []ExternalAction
	FileMap         []FileMap
	SFTP            SFTP
	URLRegexp       []string
}

func LoadConfig(filepath string) Config {
	var conf Config
	conf.InsecureCert = false
	_, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	pwd := os.Getenv("SFTP_PASSWORD")
	if pwd != "" {
		conf.SFTP.Password = pwd
	}

	return conf
}
