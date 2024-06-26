package indexer

import (
	"bufio"
	"emperror.dev/errors"
	"fmt"
	xmlparser "github.com/tamerh/xml-stream-parser"
	"golang.org/x/exp/maps"
	"io"
	"log"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

type ActionXML struct {
	server         *Server
	name           string
	format         map[string]ConfigXMLFormat
	compiledRegexp map[string]map[string]*regexp.Regexp
}

func (as *ActionXML) CanHandle(contentType string, filename string) bool {
	if strings.ToLower(filepath.Ext(filename)) == ".xml" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		//log.Printf("cannot parse media type %s", contentType)
		return false
	}
	if slices.Contains([]string{"application/xml", "text/xml", "text/plain"}, mediaType) {
		return true
	}
	return false
}

func NewActionXML(name string, format map[string]ConfigXMLFormat, server *Server, ad *ActionDispatcher) Action {
	as := &ActionXML{name: name, format: format, server: server}
	compiledRegexp := map[string]map[string]*regexp.Regexp{}
	for elem, format := range as.format {
		if _, ok := compiledRegexp[elem]; !ok {
			compiledRegexp[elem] = map[string]*regexp.Regexp{}
		}
		if format.Regexp {
			for attr, val := range format.Attributes {
				re, err := regexp.Compile(val)
				if err != nil {
					log.Printf("cannot compile regexp %s:%s: %v", elem, val, err)
					continue
				}
				compiledRegexp[elem][attr] = re
			}
		}
	}
	as.compiledRegexp = compiledRegexp

	ad.RegisterAction(as)
	return as
}

func (as *ActionXML) GetWeight() uint {
	return 10
}

func (as *ActionXML) GetCaps() ActionCapability {
	return ACTFILEHEAD | ACTSTREAM
}

func (as *ActionXML) GetName() string {
	return as.name
}

func (as *ActionXML) Stream(contentType string, reader io.Reader, filename string) (*ResultV2, error) {
	var result = NewResultV2()
	elements := maps.Keys(as.format)
	br := bufio.NewReaderSize(reader, 4096*4)
	parser := xmlparser.NewXMLParser(br, elements...).ParseAttributesOnly(elements...)
	var found bool
	for xml := range parser.Stream() {
		if xml.Err != nil {
			continue
		}
		elem := strings.ToLower(xml.Name)
		format, ok := as.format[elem]
		if !ok {
			continue
		}
		for attr, val := range xml.Attrs {
			attr = strings.ToLower(attr)
			if val2, ok := format.Attributes[attr]; !ok {
				continue
			} else {
				if format.Regexp {
					re, ok := as.compiledRegexp[elem][attr]
					if !ok {
						continue
					}
					found = re.MatchString(val)
				} else {
					found = val == val2
				}
				if found {
					result.Type = format.Type
					result.Subtype = format.Subtype
					if format.Mime != "" {
						result.Mimetypes = []string{format.Mime}
						result.Mimetype = format.Mime
					}
					if format.Pronom != "" {
						result.Pronoms = []string{format.Pronom}
						result.Pronom = format.Pronom
					}
					result.Metadata[as.GetName()] = map[string]string{
						"element":   elem,
						"attribute": fmt.Sprintf("%s=%s", attr, val),
					}
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	return result, nil
}

func (as *ActionXML) DoV2(filename string) (*ResultV2, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open file '%s'", filename)
	}
	defer reader.Close()
	return as.Stream("", reader, filename)
}

func (as *ActionXML) Do(uri *url.URL, contentType string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, []string, []string, error) {
	filename, err := as.server.fm.Get(uri)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "no file url")
	}

	fp, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "cannot open file %s", filename)
	}
	defer fp.Close()

	result, err := as.Stream("", fp, filename)
	if err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	return result.Metadata[as.GetName()], result.Mimetypes, result.Pronoms, nil
}

var (
	_ Action = &ActionXML{}
)
