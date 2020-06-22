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
package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/goph/emperror"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"html/template"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type ActionParam struct {
	Url           string   `json:"url"`
	Actions       []string `json:"actions,omitempty"`
	ForceDownload string   `json:"forcedownload,omitempty"`
	HeaderSize    int64    `json:"headersize,omitempty"`
}

type Server struct {
	srv             *http.Server
	jwtSecret       string
	jwtAlg          []string
	log             *logging.Logger
	accesslog       io.Writer
	errorTemplate   *template.Template
	actions         map[string]Action
	headerTimeout   time.Duration
	headerSize      int64
	forceDownload   string
	maxDownloadSize int64
	tempDir         string
	fm              *FileMapper
}

func NewServer(
	headerTimeout time.Duration,
	headerSize int64,
	downloadMime string,
	maxDownloadSize int64,
	jwtSecret string,
	jwtAlg []string,
	log *logging.Logger,
	accesslog io.Writer,
	errorTemplate string,
	tempDir string,
	fm *FileMapper,
) (*Server, error) {
	errorTpl, err := template.ParseFiles(errorTemplate)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot parse error template %s", errorTemplate)
	}
	srv := &Server{
		headerTimeout:   headerTimeout,
		headerSize:      headerSize,
		forceDownload:   downloadMime,
		maxDownloadSize: maxDownloadSize,
		jwtSecret:       jwtSecret,
		jwtAlg:          jwtAlg,
		log:             log,
		accesslog:       accesslog,
		tempDir:         tempDir,
		errorTemplate:   errorTpl,
		actions:         map[string]Action{},
		fm:              fm,
	}
	return srv, nil
}

func (s *Server) AddAction(a Action) {
	s.actions[a.GetName()] = a
}

func (s *Server) DoPanicf(writer http.ResponseWriter, status int, message string, a ...interface{}) (err error) {
	msg := fmt.Sprintf(message, a...)
	s.DoPanic(writer, status, msg)
	return
}

func (s *Server) DoPanic(writer http.ResponseWriter, status int, message string) (err error) {
	type errData struct {
		Status     int
		StatusText string
		Message    string
	}
	s.log.Error(message)
	data := errData{
		Status:     status,
		StatusText: http.StatusText(status),
		Message:    message,
	}
	writer.WriteHeader(status)
	// if there'ms no error Template, there's no help...
	s.errorTemplate.Execute(writer, data)
	return
}

/*
loads part of data and gets mime type
*/
func (s *Server) getContent(uri *url.URL, forceDownload string, headerSize int64, writer io.Writer) (mimetype string, fulldownload bool, err error) {
	s.log.Infof("loading from %s", uri.String())

	dlRegexp, err := regexp.Compile(forceDownload)
	if err != nil {
		return "", false, emperror.Wrapf(err, "cannot compile download mime regexp %s", forceDownload)
	}

	if uri.Scheme != "file" {
		res, err := http.Head(uri.String())
		if err != nil {
			return "", false, emperror.Wrapf(err, "error getting head request for %s", uri.String())
		}
		if res.StatusCode == http.StatusMethodNotAllowed {
			s.log.Debugf("HEAD not allowed")
			ctx, cancel := context.WithTimeout(context.Background(), s.headerTimeout)
			defer cancel() // The cancel should be deferred so resources are cleaned up
			req, err := http.NewRequestWithContext(ctx, "GET", uri.String(), nil)
			if err != nil {
				return "", false, emperror.Wrapf(err, "error creating request for %s", uri.String())
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", 64))
			var client http.Client
			res, err = client.Do(req)
			if err != nil {
				return "", false, emperror.Wrapf(err, "error querying uri")
			}
			res.Body.Close()
			if res.StatusCode >= 400 {
				return "", false, emperror.Wrapf(err, "error querying %s: %s", uri.String(), res.Status)
			}
		}
		if res.StatusCode > 300 {
			return "", false, errors.New(fmt.Sprintf("invalid status %v - %v for %s", res.StatusCode, res.StatusCode, uri.String()))
		}
		// ************************************
		// * get mimetype from response header
		// ************************************
		mimetype = ClearMime(res.Header.Get("Content-type"))
		s.log.Debugf("mimetype from server: %v", mimetype)
		fulldownload = dlRegexp.MatchString(mimetype)

		if fulldownload {
			s.log.Infof("full download of %s", uri.String())
		} else {
			s.log.Infof("downloading %v byte from %s", headerSize, uri.String())
		}

		ctx, cancel := context.WithTimeout(context.Background(), s.headerTimeout)
		defer cancel() // The cancel should be deferred so resources are cleaned up

		// build range request. we do not want to load more than needed
		req, err := http.NewRequestWithContext(ctx, "GET", uri.String(), nil)
		if err != nil {
			return "", false, emperror.Wrapf(err, "error creating request for %s", uri.String())
		}
		if !fulldownload {
			req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", headerSize-1))
		}
		var client http.Client
		resp, err := client.Do(req)
		if err != nil {
			return "", false, emperror.Wrapf(err, "error querying uri")
		}
		// default should follow redirects
		defer resp.Body.Close()

		maxSize := headerSize
		if fulldownload {
			maxSize = s.maxDownloadSize // 2 ^ 32 - 1 max. 4GB
		}
		num, err := io.CopyN(writer, resp.Body, maxSize)
		if err != nil && err != io.ErrUnexpectedEOF {
			if err.Error() != "EOF" {
				return "", false, emperror.Wrapf(err, "cannot read content from url %s", uri.String())
			}
		}
		if num == 0 {
			return "", false, errors.New(fmt.Sprintf("no content from url %s", uri.String()))
		}

	} else {
		path, err := s.fm.Get(uri)
		if err != nil {
			return "", false, emperror.Wrapf(err, "cannot map uri %s ", uri.String())
		}
		f, err := os.Open(path)
		if err != nil {
			return "", false, emperror.Wrapf(err, "cannot open file %s", path)
		}
		defer f.Close()
		buf := make([]byte, 512)
		if _, err := f.Read(buf); err != nil {
			return "", false, emperror.Wrapf(err, "cannot read from file %s", path)
		}
		mimetype = http.DetectContentType(buf)
	}

	mimetype = ClearMime(mimetype)
	return
}

func (s *Server) HandleDefault(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot read body: %v", err)
		return
	}
	param := ActionParam{ForceDownload: s.forceDownload}
	if err := json.Unmarshal(body, &param); err != nil {
		s.DoPanicf(w, http.StatusBadRequest, "cannot unmarshal json - %s: %v", string(body), err)
		return
	}

	// if no action is given, just use all
	if len(param.Actions) == 0 {
		for name, _ := range s.actions {
			param.Actions = append(param.Actions, name)
		}
	}

	result, err := s.doIndex(param)
	if err != nil {
		result = map[string]interface{}{}
		errors := map[string]string{}
		errors["index"] = err.Error()
		result["errors"] = errors
	}

	js, err := json.Marshal(result)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result %v: %v", result, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)

}

func (s *Server) doIndex(param ActionParam) (map[string]interface{}, error) {

	uri, err := url.ParseRequestURI(param.Url)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot parse url %s", param.Url)
	}

	var duration time.Duration
	var width, height uint

	tmpfile, err := ioutil.TempFile(s.tempDir, "indexer")
	if err != nil {
		return nil, emperror.Wrap(err, "cannot create tempfile")
	}
	defer os.Remove(tmpfile.Name()) // clean up

	headerSize := param.HeaderSize
	if headerSize == 0 {
		headerSize = s.headerSize
	}
	mimetype, fulldownload, err := s.getContent(uri, param.ForceDownload, headerSize, tmpfile)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot get content header of %s", uri.String())
	}
	if err := tmpfile.Close(); err != nil {
		return nil, emperror.Wrapf(err, "cannot close tempfile %s", tmpfile.Name())
	}
	tmpUri, err := url.Parse(fmt.Sprintf("file:///%s", filepath.ToSlash(tmpfile.Name())))
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot create uri for tempfile %s", tmpfile.Name())
	}

	result := map[string]interface{}{}
	errors := map[string]string{}
	// todo: download once, start concurrent identifiers...
	for key, actionstr := range param.Actions {
		action, ok := s.actions[actionstr]
		if !ok {
			// return nil, emperror.Wrapf(err, "invalid action: %s", actionstr)
			errors[actionstr] = "action not available"
			continue
		}
		theUri := uri
		caps := action.GetCaps()
		// can only deal with files
		if fulldownload || (caps&ACTFILEHEAD > 0 && caps&(ACTHTTP|ACTHTTPS) == 0) {
			if uri.Scheme != "file" {
				theUri = tmpUri
			}
		}
		s.log.Infof("Action [%v] %s: %s", key, actionstr, theUri.String())
		actionresult, err := action.Do(theUri, &mimetype, &width, &height, &duration)
		if err == ErrMimeNotApplicable {
			s.log.Infof("%s: mime %s not applicable", actionstr, mimetype)
			continue
		}
		if err != nil {
			errors[actionstr] = err.Error()
		} else {
			result[actionstr] = actionresult
		}
	}
	result["errors"] = errors
	result["mimetype"] = mimetype
	if width > 0 || height > 0 {
		result["width"] = width
		result["height"] = height
	}
	if duration > 0 {
		result["duration"] = math.Round(float64(duration) / float64(time.Second))
	}
	return result, nil
}

func (s *Server) ListenAndServe(addr, cert, key string) error {
	router := mux.NewRouter()

	router.HandleFunc("/", s.HandleDefault).Methods("POST")

	loggedRouter := handlers.LoggingHandler(s.accesslog, router)
	s.srv = &http.Server{
		Handler: loggedRouter,
		Addr:    addr,
	}
	if cert != "" && key != "" {
		s.log.Infof("starting HTTPS identification server at https://%v", addr)
		return s.srv.ListenAndServeTLS(cert, key)
	} else {
		s.log.Infof("starting HTTP identification server at http://%v", addr)
		return s.srv.ListenAndServe()
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
