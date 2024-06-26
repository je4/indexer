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
package indexer

import (
	"github.com/je4/utils/v2/pkg/checksum"
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

type TypeSubtype struct {
	Type    string
	Subtype string
}

type ConfigSiegfried struct {
	//Address string
	Enabled       bool
	SignatureFile string `toml:"signature"`
	MimeMap       map[string]string
	TypeMap       map[string]TypeSubtype
}

type ConfigTika struct {
	AddressMeta           string
	AddressFulltext       string
	Timeout               duration
	RegexpMimeFulltext    string
	RegexpMimeFulltextNot string
	RegexpMimeMeta        string
	RegexpMimeMetaNot     string
	Online                bool
	Enabled               bool
}

type FFMPEGMime struct {
	Video  bool
	Audio  bool
	Format string
	Mime   string
}

type ConfigFFMPEG struct {
	FFProbe string
	Wsl     bool
	Timeout duration
	Online  bool
	Enabled bool
	Mime    []FFMPEGMime
}

type ConfigChecksum struct {
	Name    string
	Digest  []checksum.DigestAlgorithm
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

type ConfigXMLFormat struct {
	Regexp     bool
	Attributes map[string]string
	Pronom     string
	Mime       string
	Type       string
	Subtype    string
}

type ConfigXML struct {
	Enabled bool
	Format  map[string]ConfigXMLFormat
}

type ConfigExternalAction struct {
	Name,
	Address,
	Mimetype string
	ActionCapabilities []ActionCapability
	CallType           ExternalActionCalltype
}

type ConfigFileMap struct {
	Alias  string
	Folder string
}

type ConfigSFTP struct {
	Knownhosts string
	Password   string
	PrivateKey []string
}

type ConfigNSRL struct {
	Enabled bool
	Badger  string
}

type ConfigMimeWeight struct {
	Regexp string
	Weight int
}

type IndexerConfig struct {
	Enabled         bool
	LocalCache      bool
	TempDir         string
	HeaderTimeout   duration
	HeaderSize      int64
	DownloadMime    string `toml:"forcedownload"`
	MaxDownloadSize int64
	Siegfried       ConfigSiegfried
	Checksum        ConfigChecksum
	FFMPEG          ConfigFFMPEG
	ImageMagick     ConfigImageMagick
	Tika            ConfigTika
	XML             ConfigXML
	External        []ConfigExternalAction
	FileMap         []ConfigFileMap
	URLRegexp       []string
	NSRL            ConfigNSRL
	Clamav          ConfigClamAV
	MimeRelevance   map[string]ConfigMimeWeight
}

func GetDefaultConfig() *IndexerConfig {
	var conf = &IndexerConfig{
		Enabled:    true,
		LocalCache: false,
		TempDir:    "",
		Siegfried: ConfigSiegfried{
			Enabled:       true,
			SignatureFile: "internal:/siegfried/default.sig",
			MimeMap: map[string]string{
				"x-fmt/92":  "image/psd",
				"fmt/134":   "audio/mp3",
				"x-fmt/184": "image/x-sun-raster",
				"fmt/202":   "image/x-nikon-nef",
				"fmt/211":   "image/x-photo-cd",
				"x-fmt/383": "image/fits",
				"fmt/405":   "image/x-portable-anymap",
				"fmt/406":   "image/x-portable-graymap",
				"fmt/408":   "image/x-portable-pixmap",
				"fmt/436":   "image/x-adobe-dng",
				"fmt/437":   "image/x-adobe-dng",
				"fmt/592":   "image/x-canon-cr2",
				"fmt/642":   "image/x-raw-fuji",
				"fmt/662":   "image/x-raw-panasonic",
				"fmt/668":   "image/x-olympus-orf",
				"fmt/986":   "text/xmp",
				"fmt/1001":  "image/x-exr",
				"fmt/1040":  "image/vnd.ms-dds",
				"fmt/1781":  "image/x-pentax-pef",
			},
		},
		Checksum: ConfigChecksum{
			Enabled: true,
			Name:    "checksum",
			Digest: []checksum.DigestAlgorithm{
				"sha512",
			},
		},
		FFMPEG: ConfigFFMPEG{
			Mime: []FFMPEGMime{
				{
					Video:  false,
					Audio:  true,
					Format: "mov,mp4,m4a,3gp,3g2,mj2",
					Mime:   "audio/mp4",
				},
				{
					Video:  true,
					Audio:  true,
					Format: "mov,mp4,m4a,3gp,3g2,mj2",
					Mime:   "video/mp4",
				},
				{
					Video:  true,
					Audio:  false,
					Format: "mov,mp4,m4a,3gp,3g2,mj2",
					Mime:   "video/mp4",
				},
			},
		},
		ImageMagick: ConfigImageMagick{},
		Tika:        ConfigTika{},
		External:    []ConfigExternalAction{},
		FileMap:     []ConfigFileMap{},
		URLRegexp:   []string{},
		NSRL:        ConfigNSRL{},
		Clamav:      ConfigClamAV{},
		MimeRelevance: map[string]ConfigMimeWeight{
			"1": {
				Regexp: "^application/octet-stream",
				Weight: 1,
			},
			"2": {
				Regexp: "^text/plain",
				Weight: 3,
			},
			"3": {
				Regexp: "^audio/mpeg",
				Weight: 6,
			},
			"4": {
				Regexp: "^video/mpeg",
				Weight: 5,
			},
			"5": {
				Regexp: "^application/vnd\\..+",
				Weight: 4,
			},
			"6": {
				Regexp: "^application/rtf",
				Weight: 4,
			},
			"7": {
				Regexp: "^application/.+",
				Weight: 2,
			},
			"8": {
				Regexp: "^text/.+",
				Weight: 4,
			},
			"9": {
				Regexp: "^audio/.+",
				Weight: 5,
			},
			"10": {
				Regexp: "^video/.+",
				Weight: 4,
			},
			"11": {
				Regexp: "^.+/x-.+",
				Weight: 80,
			},
		},
	}
	return conf
}
