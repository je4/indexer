package indexer

import (
	"emperror.dev/errors"
	iou "github.com/je4/utils/v2/pkg/io"
	"golang.org/x/exp/slices"
	"io"
	"regexp"
	"sort"
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

func (ad *ActionDispatcher) Stream(reader io.Reader, filename string) (*ResultV2, error) {
	var writer = []*iou.WriteIgnoreCloser{}
	wg := sync.WaitGroup{}
	/*
		mimeReader, err := iou.NewMimeReader(reader)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot create MimeReader for %s", filename)
		}
		mimeType, _ := mimeReader.DetectContentType()
		dataType := mimeType[:strings.IndexByte(mimeType, '/')]
	*/
	dataType := ""
	mimeReader := reader
	results := make(chan *ResultV2, len(ad.actions))
	for _, action := range ad.actions {
		if action.GetCaps()&ACTSTREAM != 0 {
			wg.Add(1)
			pr, pw := io.Pipe()
			writer = append(writer, iou.NewWriteIgnoreCloser(pw))
			go func(r io.Reader, a Action) {
				// stream to actions
				result, err := a.Stream(dataType, r, filename)
				if err != nil {
					result = NewResultV2()
					result.Errors[a.GetName()] = err.Error()
				}
				// send result to channel
				if result != nil {
					results <- result
				}
				// discard remaining data
				_, _ = io.Copy(io.Discard, r)
				wg.Done()
			}(iou.NewReadIgnoreCloser(pr), action)
		}
	}
	var ws = []io.Writer{}
	for _, w := range writer {
		ws = append(ws, w)
	}
	multiWriter := io.MultiWriter(ws...)
	written, err := io.Copy(multiWriter, mimeReader)
	for _, w := range writer {
		w.ForceClose()
	}
	if err != nil {
		return nil, errors.Wrap(err, "cannot copy stream to actions")
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
	slices.SortFunc(result.Mimetypes, func(a, b string) bool {
		// higher weight means less in sorting
		return mimeMap[a] > mimeMap[b]
	})
	if len(result.Mimetypes) > 0 {
		result.Mimetype = result.Mimetypes[0]
	}
	if len(result.Pronoms) > 0 {
		result.Pronom = result.Pronoms[0]
	}

	result.Size = uint64(written)
	return result, nil
}
