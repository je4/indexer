// Package indexer
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
	"context"
	"crypto/tls"
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	mime "github.com/gabriel-vasile/mimetype"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"html/template"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type ActionParam struct {
	Url           string            `json:"url"`
	Actions       []string          `json:"actions,omitempty"`
	ForceDownload string            `json:"forcedownload,omitempty"`
	HeaderSize    int64             `json:"headersize,omitempty"`
	Checksums     map[string]string `json:"checksums,omitempty"`
}

type MimeWeightString struct {
	Regexp string
	Weight int
}
type MimeWeight struct {
	regexp *regexp.Regexp
	weight int
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
	sftp            *SFTP
	insecureCert    bool
	mimeRelevance   []MimeWeight
}

func NewServer(
	headerTimeout time.Duration,
	headerSize int64,
	downloadMime string,
	maxDownloadSize int64,
	mimeRelevance map[int]MimeWeightString,
	jwtSecret string,
	jwtAlg []string,
	insecureCert bool,
	log *logging.Logger,
	accesslog io.Writer,
	errorTemplate *template.Template,
	tempDir string,
	fm *FileMapper,
	sftp *SFTP,
) (*Server, error) {
	srv := &Server{
		headerTimeout:   headerTimeout,
		headerSize:      headerSize,
		insecureCert:    insecureCert,
		forceDownload:   downloadMime,
		maxDownloadSize: maxDownloadSize,
		jwtSecret:       jwtSecret,
		jwtAlg:          jwtAlg,
		log:             log,
		accesslog:       accesslog,
		tempDir:         tempDir,
		errorTemplate:   errorTemplate,
		actions:         map[string]Action{},
		fm:              fm,
		sftp:            sftp,
		mimeRelevance:   []MimeWeight{},
	}
	mKeys := []int{}
	for key, _ := range mimeRelevance {
		mKeys = append(mKeys, key)
	}
	sort.Ints(mKeys)
	for _, key := range mKeys {
		rexp, err := regexp.Compile(mimeRelevance[key].Regexp)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot compile Regexp %s", key)
		}
		srv.mimeRelevance = append(srv.mimeRelevance, MimeWeight{
			regexp: rexp,
			weight: mimeRelevance[key].Weight,
		})
	}
	return srv, nil
}

func (s *Server) AddAction(a Action) {
	s.actions[a.GetName()] = a
}

/*
holistic function to give some mimetypes a relevance
*/
func (s *Server) MimeRelevance(mimetype string) (relevance int) {
	if mimetype == "" {
		return 0
	}
	for _, val := range s.mimeRelevance {
		if val.regexp.MatchString(mimetype) {
			return val.weight
		}
	}
	/*
		if mimetype == "application/octet-stream" {
			return 1
		}
		if mimetype == "text/plain" {
			return 2
		}
		if mimetype == "audio/mpeg" {
			return 2
		}
		if mimetype == "video/mpeg" {
			return 2
		}
		if strings.HasPrefix(mimetype, "application/") {
			return 3
		}
		if strings.HasPrefix(mimetype, "text/") {
			return 4
		}
	*/
	return 100
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

func (s *Server) getMimeHTTP(uri *url.URL) (string, error) {
	customTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return "", fmt.Errorf("http.DefaultTransport no (*http.Transport)")
	}
	customTransport = customTransport.Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: s.insecureCert}
	client := &http.Client{Transport: customTransport}

	res, err := client.Head(uri.String())
	if err != nil {
		return "", errors.Wrapf(err, "error getting head request for %s", uri.String())
	}
	if res.StatusCode == http.StatusMethodNotAllowed || res.StatusCode == http.StatusForbidden {
		s.log.Debugf("HEAD not allowed")
		ctx, cancel := context.WithTimeout(context.Background(), s.headerTimeout)
		defer cancel() // The cancel should be deferred so resources are cleaned up
		req, err := http.NewRequestWithContext(ctx, "GET", uri.String(), nil)
		if err != nil {
			return "", errors.Wrapf(err, "error creating request for %s", uri.String())
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", 64))
		var client http.Client
		res, err = client.Do(req)
		if err != nil {
			return "", errors.Wrapf(err, "error querying uri")
		}
		res.Body.Close()
		if res.StatusCode >= 400 {
			return "", errors.Wrapf(err, "error querying %s: %s", uri.String(), res.Status)
		}
	}
	if res.StatusCode > 300 {
		return "", errors.New(fmt.Sprintf("invalid status %v - %v for %s", res.StatusCode, res.StatusCode, uri.String()))
	}
	// ************************************
	// * get mimetype from response header
	// ************************************
	return ClearMime(res.Header.Get("Content-type")), nil
}

func (s *Server) loadHTTP(uri *url.URL, writer io.Writer, fulldownload bool) (int64, error) {
	customTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return 0, fmt.Errorf("http.DefaultTransport no (*http.Transport)")
	}
	customTransport = customTransport.Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: s.insecureCert}
	client := &http.Client{Transport: customTransport}

	ctx, cancel := context.WithTimeout(context.Background(), s.headerTimeout)
	defer cancel() // The cancel should be deferred so resources are cleaned up

	// build range request. we do not want to load more than needed
	req, err := http.NewRequestWithContext(ctx, "GET", uri.String(), nil)
	if err != nil {
		return 0, errors.Wrapf(err, "error creating request for %s", uri.String())
	}
	if !fulldownload {
		req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", s.headerSize-1))
	}
	//var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return 0, errors.Wrapf(err, "error querying uri")
	}
	// default should follow redirects
	defer resp.Body.Close()

	maxSize := s.headerSize
	if fulldownload {
		maxSize = s.maxDownloadSize // 2 ^ 32 - 1 max. 4GB
	}
	num, err := io.CopyN(writer, resp.Body, maxSize)
	if err != nil && err != io.ErrUnexpectedEOF {
		if err.Error() != "EOF" {
			return 0, errors.Wrapf(err, "cannot read content from url %s", uri.String())
		}
	}
	if num == 0 {
		return 0, errors.New(fmt.Sprintf("no content from url %s", uri.String()))
	}
	return num, nil
}

/*
func (s *Server) loadSFTP(uri *url.URL, writer io.Writer) (int64, error) {

}

*/

/*
loads part of data and gets mime type
*/
func (s *Server) getContent(uri *url.URL, forceDownloadRegexp *regexp.Regexp) (mimetype string, fulldownload bool, tmpfile *os.File, err error) {
	s.log.Infof("loading from %s", uri.String())

	if uri.Scheme == "http" || uri.Scheme == "https" {
		mimetype, err = s.getMimeHTTP(uri)
		if err != nil {
			return "", false, nil, errors.Wrapf(err, "error loading mime from %s", uri.String())
		}
		s.log.Debugf("mimetype from server: %v", mimetype)
		fulldownload = forceDownloadRegexp.MatchString(mimetype)

		if fulldownload {
			s.log.Infof("full download of %s", uri.String())
		} else {
			s.log.Infof("downloading %v byte from %s", s.headerSize, uri.String())
		}
		tmpfile, err = os.CreateTemp(s.tempDir, "indexer")
		if err != nil {
			return "", false, nil, errors.Wrap(err, "cannot create tempfile")
		}

		if _, err = s.loadHTTP(uri, tmpfile, fulldownload); err != nil {
			return "", false, nil, errors.Wrapf(err, "error loading from web %s", uri.String())
		}
	} else if uri.Scheme == "sftp" {
		tmpfile, err = os.CreateTemp(s.tempDir, "indexer")
		if err != nil {
			return "", false, nil, errors.Wrap(err, "cannot create tempfile")
		}

		_, err := s.sftp.Get(*uri, tmpfile)
		fulldownload = true
		if err != nil {
			return "", false, nil, errors.Wrapf(err, "error loading from sftp %s", uri.String())
		}
	} else {
		fulldownload = true
		path, err := s.fm.Get(uri)
		if err != nil {
			return "", false, nil, errors.Wrapf(err, "cannot map uri %s ", uri.String())
		}
		f, err := os.Open(path)
		if err != nil {
			return "", false, nil, errors.Wrapf(err, "cannot open file %s", path)
		}
		defer f.Close()
		buf := make([]byte, 512)
		if _, err := f.Read(buf); err != nil {
			if err != io.EOF {
				return "", false, nil, errors.Wrapf(err, "cannot read from file %s", path)
			}
		}
		mimetype = http.DetectContentType(buf)
	}

	mimetype = ClearMime(mimetype)
	return
}

var unibasSFTPRegexp = regexp.MustCompile("^sftp:/([^/].+)$")

func (s *Server) HandleDefault(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot read body: %v", err)
		return
	}
	param := ActionParam{ForceDownload: s.forceDownload, Checksums: map[string]string{}}
	if err := json.Unmarshal(body, &param); err != nil {
		s.DoPanicf(w, http.StatusBadRequest, "cannot unmarshal json - %s: %v", string(body), err)
		return
	}
	param.Url, err = url.QueryUnescape(param.Url)
	if err != nil {
		s.DoPanicf(w, http.StatusBadRequest, "cannot unescape url - %s: %v", param.Url, err)
		return
	}

	// if no action is given, just use all
	if len(param.Actions) == 0 {
		for name, _ := range s.actions {
			param.Actions = append(param.Actions, name)
		}
	}

	// todo: bad code. make it configurable
	str := unibasSFTPRegexp.FindStringSubmatch(param.Url)
	if len(str) > 1 {
		param.Url = fmt.Sprintf("sftp://mb_sftp@mb-wf2.memobase.unibas.ch:80/%s", str[1])
	}

	result, err := s.doIndex(param)
	if err != nil {
		result = map[string]interface{}{}
		errors := map[string]string{}
		errors["index"] = err.Error()
		result["errors"] = errors
		s.log.Errorf("error on indexing: %v", err)
	}

	js, err := json.Marshal(result)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result %v: %v", result, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)

}

var fileUrlRegexp = regexp.MustCompile("^file://([^/]*)/(.+)$")

func (s *Server) doIndex(param ActionParam) (map[string]interface{}, error) {
	var err error
	matches := fileUrlRegexp.FindStringSubmatch(param.Url)
	var uri *url.URL
	if matches != nil {
		ustr := fmt.Sprintf("file://%s/%s", matches[1], url.PathEscape(strings.TrimLeft(matches[2], "/")))
		uri, err = url.ParseRequestURI(ustr)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse url %s", ustr)
		}
	} else {
		uri, err = url.Parse(param.Url)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse url %s", param.Url)
		}
	}
	var duration time.Duration
	var width, height uint

	headerSize := param.HeaderSize
	if headerSize == 0 {
		headerSize = s.headerSize
	}
	forceDownload := param.ForceDownload
	forceDownloadRegexp, err := regexp.Compile(forceDownload)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot compile forcedownload Regexp %v", param.ForceDownload)
	}

	mimetype, fulldownload, tmpfile, err := s.getContent(uri, forceDownloadRegexp)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get content header of %s", uri.String())
	}
	if tmpfile != nil {
		if err := tmpfile.Close(); err != nil {
			return nil, errors.Wrapf(err, "cannot close tempfile %s", tmpfile.Name())
		}
		defer func() {
			name := tmpfile.Name()
			os.Remove(name) // clean up
		}()
	}

	var tmpUri *url.URL
	if tmpfile != nil {
		if mimetype == "" && fulldownload {
			m, err := mime.DetectFile(tmpfile.Name())
			if err != nil {
				return nil, errors.Wrapf(err, "cannot detect mimetype of %v", tmpfile.Name())
			}
			mimetype = ClearMime(m.String())
		}
		tmpUri, err = url.Parse(fmt.Sprintf("file:///%s", filepath.ToSlash(tmpfile.Name())))
		if err != nil {
			return nil, errors.Wrapf(err, "cannot create uri for tempfile %s", tmpfile.Name())
		}
	}
	result := map[string]interface{}{}
	errors := map[string]string{}
	// todo: download once, start concurrent identifiers...
	for key, actionstr := range param.Actions {
		action, ok := s.actions[actionstr]
		if !ok {
			// return nil, errors.Wrapf(err, "invalid action: %s", actionstr)
			errors[actionstr] = "action not available"
			continue
		}
		theUri := uri
		caps := action.GetCaps()
		// can only deal with files
		if fulldownload || (caps&ACTFILEHEAD > 0 && caps&(ACTHTTP|ACTHTTPS) == 0) {
			if tmpfile != nil /* uri.Scheme != "file" */ {
				theUri = tmpUri
			}
		}
		if !fulldownload && (caps&(^ACTFILEFULL)) == 0 {
			s.log.Infof("%s: no full download. action not applicable", actionstr)
			errors[actionstr] = fmt.Errorf("no full download. action not applicable").Error()
			continue
		}
		s.log.Infof("Action [%v] %s: %s", key, actionstr, theUri.String())
		actionresult, _, err := action.Do(theUri, &mimetype, &width, &height, &duration, param.Checksums)
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
