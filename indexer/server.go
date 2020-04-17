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
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ActionParam struct {
	Url     string   `json:url`
	Actions []string `json:actions,omitempty`
}

type Server struct {
	srv           *http.Server
	jwtSecret     string
	jwtAlg        []string
	log           *logging.Logger
	accesslog     io.Writer
	errorTemplate *template.Template
	actions       map[string]Action
	headerTimeout time.Duration
	headerSize    uint
	tempDir       string
}

func NewServer(
	headerTimeout time.Duration,
	headerSize uint,
	jwtSecret string,
	jwtAlg []string,
	log *logging.Logger,
	accesslog io.Writer,
	errorTemplate string,
	tempDir string,
) *Server {
	srv := &Server{
		headerTimeout: headerTimeout,
		headerSize:    headerSize,
		jwtSecret:     jwtSecret,
		jwtAlg:        jwtAlg,
		log:           log,
		accesslog:     accesslog,
		tempDir:       tempDir,
		errorTemplate: template.Must(template.ParseFiles(errorTemplate)),
		actions:       map[string]Action{},
	}
	return srv
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
func (s *Server) getContentHeader(uri *url.URL) (buf []byte, mimetype string, err error) {
	s.log.Infof("loading header from %s", uri.String())

	if uri.Scheme != "file" {
		ctx, cancel := context.WithTimeout(context.Background(), s.headerTimeout)
		defer cancel() // The cancel should be deferred so resources are cleaned up

		// build range request. we do not want to load more than needed
		req, err := http.NewRequestWithContext(ctx, "GET", uri.String(), nil)
		if err != nil {
			return nil, "", emperror.Wrapf(err, "error creating request for uri", uri.String())
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", s.headerSize-1))
		var client http.Client
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", emperror.Wrapf(err, "error querying uri")
		}
		// default should follow redirects
		defer resp.Body.Close()

		// read head of content
		buf = make([]byte, s.headerSize)
		num, err := io.ReadFull(resp.Body, buf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, "", emperror.Wrapf(err, "cannot read content from url %s", uri.String())
		}
		if num == 0 {
			return nil, "", errors.New(fmt.Sprintf("no content from url %s", uri.String()))
		}

		// ************************************
		// * get mimetype from response header
		// ************************************
		mimetype = resp.Header.Get("Content-type")
	} else {
		path, err := getFilePath(uri)
		if err != nil {
			return nil, "", emperror.Wrapf(err, "cannot map uri %s ", uri.String())
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, "", emperror.Wrapf(err, "cannot open file %s", path)
		}
		buf = make([]byte, s.headerSize)
		if _, err := f.Read(buf); err != nil {
			return nil, "", emperror.Wrapf(err, "cannot read from file %s", path)
		}
		mimetype = http.DetectContentType(buf)
	}

	// try to get a clean mimetype
	for _, v := range strings.Split(mimetype, ",") {
		t, _, err := mime.ParseMediaType(v)
		if err != nil {
			continue
		}
		mimetype = t
		break
	}
	return
}

func (s *Server) HandleDefault(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot read body: %v", err)
		return
	}
	param := ActionParam{}
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

	uri, err := url.ParseRequestURI(param.Url)
	if err != nil {
		s.DoPanicf(w, http.StatusBadRequest, "cannot parse url - %s: %v", param.Url, err)
		return
	}

	var duration time.Duration
	var width, height uint

	buf, mimetype, err := s.getContentHeader(uri)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot get content header of %s: %v", uri.String(), err)
		return
	}
	// ************************************
	// * write data into file
	// ************************************
	// write buf to temp file
	tmpfile, err := ioutil.TempFile(s.tempDir, "indexer")
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot create tempfile: %v", err)
		return
	}
	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write(buf); err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot write to tempfile %s: %v", tmpfile.Name(), err)
		return
	}
	if err := tmpfile.Close(); err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot close tempfile %s: %v", tmpfile.Name(), err)
		return
	}
	tmpUri, err := url.Parse(fmt.Sprintf("file:///%s", filepath.ToSlash(tmpfile.Name())))
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot create uri for tempfile %s: %v", tmpfile.Name(), err)
		return
	}


	result := map[string]interface{}{}
	errors := map[string]string{}
	// todo: download once, start concurrent identifiers...
	for key, actionstr := range param.Actions {
		s.log.Infof("Action %v: %s", key, actionstr)
		action, ok := s.actions[actionstr]
		if !ok {
			s.DoPanicf(w, http.StatusBadRequest, "invalid action: %s", actionstr)
			return
		}
		theUri := uri
		caps := action.GetCaps()
		// can only deal with files
		if  caps & ACTFILEHEAD > 0 && caps & (ACTHTTP | ACTHTTPS) == 0 {
			if uri.Scheme != "file" {
				theUri = tmpUri
			}
		}
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
		result["duration"] = math.Round(float64(duration)/float64(time.Second))
	}


	js, err := json.Marshal(result)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result %v: %v", result, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
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
