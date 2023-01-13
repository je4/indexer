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
	"emperror.dev/errors"
	"fmt"
	"github.com/op/go-logging"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var _logformat = logging.MustStringFormatter(
	`%{time:2006-01-02T15:04:05.000} %{module}::%{shortfunc} [%{shortfile}] > %{level:.5s} - %{message}`,
)

func Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func CreateLogger(module string, logfile string, loglevel string) (log *logging.Logger, lf *os.File) {
	log = logging.MustGetLogger(module)
	var err error
	if logfile != "" {
		lf, err = os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Errorf("Cannot open logfile %v: %v", logfile, err)
		}
		//defer lf.CloseInternal()

	} else {
		lf = os.Stderr
	}
	backend := logging.NewLogBackend(lf, "", 0)
	backendLeveled := logging.AddModuleLevel(backend)
	backendLeveled.SetLevel(logging.GetLevel(loglevel), "")

	logging.SetFormatter(_logformat)
	logging.SetBackend(backendLeveled)

	return
}

func ClearMime(mimetype string) string {
	// try to get a clean mimetype
	for _, v := range strings.Split(mimetype, ",") {
		t, _, err := mime.ParseMediaType(v)
		if err != nil {
			continue
		}
		return t
		break
	}
	return mimetype

}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func _getFilePath(uri *url.URL) (string, error) {
	if uri.Scheme != "file" {
		return "", errors.New(fmt.Sprintf("invalid url scheme: %s", uri.Scheme))
	}
	filename := filepath.Clean(uri.Path)
	if runtime.GOOS == "windows" {
		filename = strings.TrimLeft(filename, string(filepath.Separator))
	}
	return filename, nil
}

var regexpDriveLetter = regexp.MustCompile("^([A-Za-z]):/(.*$)")

func pathToWSL(path string) string {
	matches := regexpDriveLetter.FindStringSubmatch(filepath.ToSlash(path))
	// no drive letter
	if matches == nil {
		return path
	}
	return fmt.Sprintf("/mnt/%s/%s", strings.ToLower(matches[1]), matches[2])
}
