package indexer

import (
	"bufio"
	"bytes"
	"cmp"
	"emperror.dev/errors"
	iou "github.com/je4/utils/v2/pkg/io"
	"golang.org/x/exp/slices"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type ActionDispatcher struct {
	mimeRelevance []MimeWeight
	actions       map[string]Action
}

func NewActionDispatcher(mimeRelevance map[int]MimeWeightString) *ActionDispatcher {
	ad := &ActionDispatcher{
		mimeRelevance: []MimeWeight{},
		actions:       map[string]Action{},
	}
	for _, mime := range mimeRelevance {
		ad.mimeRelevance = append(ad.mimeRelevance, MimeWeight{
			weight: mime.Weight,
			regexp: regexp.MustCompile(mime.Regexp),
		})
	}
	return ad
}

func (ad *ActionDispatcher) GetActions() map[string]Action {
	return ad.actions
}

func (ad *ActionDispatcher) Sort(actions []string) {
	sort.SliceStable(actions, func(i, j int) bool {
		return ad.actions[actions[i]].GetWeight() < ad.actions[actions[j]].GetWeight()
	})
}

func (ad *ActionDispatcher) RegisterAction(action Action) {
	ad.actions[action.GetName()] = action
}

func (ad *ActionDispatcher) GetAction(name string) (Action, bool) {
	action, ok := ad.actions[name]
	return action, ok
}

func (ad *ActionDispatcher) GetActionNames() []string {
	var names []string
	for name := range ad.actions {
		names = append(names, name)
	}
	return names
}

func (ad *ActionDispatcher) GetActionNamesByCaps(caps ActionCapability) []string {
	var names []string
	for name, action := range ad.actions {
		if action.GetCaps()&caps != 0 {
			names = append(names, name)
		}
	}
	return names
}

func (ad *ActionDispatcher) Stream(sourceReader io.Reader, stateFiles []string, actions []string) (*ResultV2, error) {
	if len(stateFiles) == 0 {
		stateFiles = []string{""}
	}
	mimeReader, err := iou.NewMimeReader(sourceReader)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create MimeReader for %s", stateFiles)
	}
	contentType, _ := mimeReader.DetectContentType()
	parts := strings.Split(contentType, ";")
	contentType = parts[0]

	var actionWriters = []*iou.WriteIgnoreCloser{}
	var wg = sync.WaitGroup{}
	results := make(chan *ResultV2, len(ad.actions))
	for _, actionStr := range actions {
		var found bool
		for _, action := range ad.actions {
			if actionStr == action.GetName() && action.GetCaps()&ACTSTREAM != 0 {
				found = true
				if !action.CanHandle(contentType, stateFiles[0]) {
					continue
				}
				wg.Add(1)
				pr, pw := io.Pipe()
				actionWriters = append(actionWriters, iou.NewWriteIgnoreCloser(pw))
				go func(actionReader io.Reader, a Action) {
					defer wg.Done()
					// stream to actions
					result, err := a.Stream(contentType, actionReader, stateFiles[0])
					if err != nil {
						result = NewResultV2()
						result.Errors[a.GetName()] = err.Error()
					}
					// send result to channel
					if result != nil {
						results <- result
					}
					// discard remaining data
					_, _ = io.Copy(io.Discard, actionReader)
				}(iou.NewReadIgnoreCloser(pr), action)
			}
		}
		if !found {
			return nil, errors.Errorf("action '%s' not configured", actionStr)
		}
	}
	var actionBufferWriters = []io.Writer{}
	for _, w := range actionWriters {
		actionBufferWriters = append(actionBufferWriters, bufio.NewWriterSize(w, 1024*1024))
		//		ws = append(ws, w)
	}
	multiWriter := io.MultiWriter(actionBufferWriters...)
	errorList := []error{}
	written, err := io.Copy(multiWriter, mimeReader)
	if err != nil {
		errorList = append(errorList, errors.Wrap(err, "cannot copy mimereader to actionwriter"))
	}
	for _, actionBufferWriter := range actionBufferWriters {
		// it's sure, that w is a bufio.Writer
		if err := actionBufferWriter.(*bufio.Writer).Flush(); err != nil {
			errorList = append(errorList, errors.Wrap(err, "cannot flush buffer"))
		}
	}
	for _, w := range actionWriters {
		if err := w.ForceClose(); err != nil {
			errorList = append(errorList, errors.Wrap(err, "cannot close buffer"))
		}
	}
	// error of copy
	if len(errorList) > 0 {
		return nil, errors.Wrap(errors.Combine(errorList...), "cannot copy stream to actions")
	}
	// wait for all actions to finish
	wg.Wait()
	close(results)
	result := NewResultV2()
	for r := range results {
		result.Merge(r)
	}

	// sort mimetypes by weight
	slices.Sort(result.Mimetypes)
	result.Mimetypes = slices.Compact(result.Mimetypes)
	mimeMap := map[string]int{}
	for _, mimetype := range result.Mimetypes {
		mimeMap[mimetype] = 50
		for _, mr := range ad.mimeRelevance {
			if mr.regexp.MatchString(mimetype) {
				mimeMap[mimetype] = mr.weight
			}
		}
	}
	slices.SortFunc(result.Mimetypes, func(a, b string) int {
		// higher weight means less in sorting
		return cmp.Compare(mimeMap[a], mimeMap[b])
	})
	if len(result.Mimetypes) > 0 {
		result.Mimetype = result.Mimetypes[0]
	}
	if len(result.Pronoms) > 0 {
		result.Pronom = result.Pronoms[0]
	}

	if result.Type == "" {
		idx := strings.IndexByte(result.Mimetype, ':')
		if idx >= 0 {
			result.Type = result.Mimetype[:idx]
		}
	}

	result.Size = uint64(written)
	return result, nil
}

func (ad *ActionDispatcher) DoV2(filename string, stateFiles []string, actions []string) (*ResultV2, error) {
	if len(stateFiles) == 0 {
		stateFiles = append(stateFiles, "")
	}

	fp, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open '%s'", filename)
	}
	data := bytes.Buffer{}
	w := bufio.NewWriter(&data)
	if _, err := io.CopyN(w, fp, 512); err != nil {
		return nil, errors.Wrapf(err, "cannot read file '%s'", filename)
	}
	contentType := http.DetectContentType(data.Bytes())

	results := &ResultV2{
		Errors:    map[string]string{},
		Mimetype:  contentType,
		Mimetypes: []string{contentType},
		Pronom:    "",
		Pronoms:   []string{},
		Width:     0,
		Height:    0,
		Duration:  0,
		Size:      0,
		Metadata:  map[string]any{},
	}
	for _, actionStr := range actions {
		var found bool
		for _, action := range ad.actions {
			if actionStr == action.GetName() && action.GetCaps()&ACTSTREAM != 0 {
				found = true
				if !action.CanHandle(results.Mimetype, filename) {
					break
				}
				// stream to actions
				result, err := action.DoV2(filename)
				if err != nil {
					result = NewResultV2()
					result.Errors[action.GetName()] = err.Error()
				}
				// send result to channel
				if result != nil {
					results.Merge(result)
				}
				break
			}
		}
		if !found {
			return nil, errors.Errorf("action '%s' not configured", actionStr)
		}
	}

	// sort mimetypes by weight
	slices.Sort(results.Mimetypes)
	results.Mimetypes = slices.Compact(results.Mimetypes)
	mimeMap := map[string]int{}
	for _, mimetype := range results.Mimetypes {
		mimeMap[mimetype] = 50
		for _, mr := range ad.mimeRelevance {
			if mr.regexp.MatchString(mimetype) {
				mimeMap[mimetype] = mr.weight
			}
		}
	}
	slices.SortFunc(results.Mimetypes, func(a, b string) int {
		// higher weight means less in sorting
		return cmp.Compare(mimeMap[a], mimeMap[b])
	})
	if len(results.Mimetypes) > 0 {
		results.Mimetype = results.Mimetypes[0]
	}
	if len(results.Pronoms) > 0 {
		results.Pronom = results.Pronoms[0]
	}

	fi, err := os.Stat(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot stat '%s'", filename)
	}
	results.Size = uint64(fi.Size())
	return results, nil
}
