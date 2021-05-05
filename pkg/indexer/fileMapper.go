// This file is part of Memobase Mediaserver which is released under GPLv3.
// See file license.txt for full license details.
//
// Author Juergen Enge <juergen@info-age.net>
//
// This code uses elements from
// * "Mediaserver" (Center for Digital Matter HGK FHNW, Basel)
// * "Remote Exhibition Project" (info-age GmbH, Basel)
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
