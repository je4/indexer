package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/goph/emperror"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

// java -jar tika-server-1.24.jar -enableUnsecureFeatures -enableFileUrl --port=9997

type ActionTika struct {
	name string
	url  string
	timeout time.Duration
	regexpMime *regexp.Regexp
}

func NewActionTika(uri string, timeout time.Duration, regexpMime string) Action {

	return &ActionTika{
		name: "tika",
		url: uri,
		timeout: timeout,
		regexpMime: regexp.MustCompile(regexpMime),
	}
}

func (at *ActionTika) GetCaps() ActionCapability {
	return ACTFILEHEAD
}

func (at *ActionTika) GetName() string {
	return at.name
}

func (at *ActionTika) Do(uri *url.URL, mimetype *string, width *uint, height *uint, duration *time.Duration) (interface{}, error) {
	if !at.regexpMime.MatchString(*mimetype) {
		return nil, ErrMimeNotApplicable
	}

	var dataOut io.Reader
	// local files need some adjustments...
	if uri.Scheme == "file" {
		filename, err := getFilePath(uri)
		if err != nil {
			return nil, emperror.Wrapf(err, "invalid file uri %s", uri.String())
		}
		f, err := os.Open(filename)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot open: %s", filename)
		}
		defer f.Close()
		dataOut = f
	} else {
		//		filename = uri.String()
		resp, err := http.Get(uri.String())
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot load url: %s", uri.String())
		}
		defer resp.Body.Close()
		dataOut = resp.Body
	}


	client := &http.Client{}
	ctx, cancel := context.WithTimeout(context.Background(), at.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, at.url, dataOut)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot create tika request - %v", at.url)
	}
	req.Header.Add("Accept", "application/json")
	//req.Header.Add("fileUrl", uri.String())
	tresp, err := client.Do(req)
	if err != nil {
		return nil, emperror.Wrapf(err, "error in tika request - %v", at.url)
	}
	defer tresp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(tresp.Body)
	if err != nil {
		return nil, emperror.Wrapf(err, "error reading body - %v", at.url)
	}

	if tresp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("status not ok - %v -> %v: %s", at.url, tresp.Status, string(bodyBytes)))
	}

	result := make(map[string]interface{})
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, emperror.Wrapf(err, "error decoding json - %v", string(bodyBytes))
	}
	return result, nil
}


