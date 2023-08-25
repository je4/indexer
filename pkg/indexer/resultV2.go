package indexer

import "golang.org/x/exp/slices"

type ResultV2 struct {
	Errors    map[string]string `json:"errors,omitempty"`
	Mimetype  string            `json:"mimetype"`
	Mimetypes []string          `json:"mimetypes"`
	Pronom    string            `json:"pronom"`
	Pronoms   []string          `json:"pronoms"`
	Checksum  map[string]string `json:"checksum"`
	Width     uint              `json:"width,omitempty"`
	Height    uint              `json:"height,omitempty"`
	Duration  uint              `json:"duration,omitempty"`
	Size      uint64            `json:"size"`
	Metadata  map[string]any    `json:"metadata"`
	Type      string            `json:"type"`
	Subtype   string            `json:"subtype"`
}

func NewResultV2() *ResultV2 {
	return &ResultV2{
		Errors:    map[string]string{},
		Mimetypes: []string{},
		Pronoms:   []string{},
		Checksum:  map[string]string{},
		Metadata:  map[string]any{},
	}
}

func (v *ResultV2) Merge(r *ResultV2) {
	if r == nil {
		return
	}
	if r.Mimetypes != nil {
		v.Mimetypes = append(v.Mimetypes, r.Mimetypes...)
		slices.Sort(v.Mimetypes)
		v.Mimetypes = slices.Compact(v.Mimetypes)
	}
	if r.Pronoms != nil {
		v.Pronoms = append(v.Pronoms, r.Pronoms...)
		slices.Sort(v.Pronoms)
		v.Pronoms = slices.Compact(v.Pronoms)
	}
	for key, val := range r.Checksum {
		v.Checksum[key] = val
	}
	if r.Mimetype != "" {
		v.Mimetype = r.Mimetype
	}
	if r.Pronom != "" {
		v.Pronom = r.Pronom
	}
	if r.Width > v.Width {
		v.Width = r.Width
	}
	if r.Height > v.Height {
		v.Height = r.Height
	}
	if r.Duration > v.Duration {
		v.Duration = r.Duration
	}
	if r.Size > v.Size {
		v.Size = r.Size
	}
	for k, m := range r.Metadata {
		v.Metadata[k] = m
	}
	if r.Errors != nil {
		if v.Errors == nil {
			v.Errors = map[string]string{}
		}
		for k, e := range r.Errors {
			v.Errors[k] = e
		}
	}
	if r.Type != "" {
		v.Type = r.Type
		v.Subtype = r.Subtype
	}
}

type FullMagickResult struct {
	Magick *MagickResult `json:"magick"`
	Frames []*Geometry   `json:"frames,omitempty"`
}
