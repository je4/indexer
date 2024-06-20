package indexer

import (
	"encoding/xml"
	"github.com/je4/indexer/v3/data"
)

type MimeBase struct {
	Mimemap Mimemap `json:"mimemap" xml:"mimemap"`
}

type Mimemap struct {
	MIME []MIME `json:"mime" xml:"mime"`
}

type MIME struct {
	Type        string    `json:"_type" xml:"type,attr"`
	Acronym     *string   `json:"_acronym,omitempty" xml:"acronym,attr,omitempty"`
	Description *string   `json:"_description,omitempty" xml:"description,attr,omitempty"`
	Priority    *string   `json:"_priority,omitempty" xml:"priority,attr,omitempty"`
	Pattern     *string   `json:"_pattern,omitempty" xml:"pattern,attr,omitempty"`
	DataType    *DataType `json:"_data-type,omitempty" xml:"data-type,attr,omitempty"`
	Offset      *string   `json:"_offset,omitempty" xml:"offset,attr,omitempty"`
	Magic       *string   `json:"_magic,omitempty" xml:"magic,attr,omitempty"`
	Endian      *Endian   `json:"_endian,omitempty" xml:"endian,attr,omitempty"`
	Mask        *string   `json:"_mask,omitempty" xml:"mask,attr,omitempty"`
}

type DataType string

const (
	Byte   DataType = "byte"
	Long   DataType = "long"
	Short  DataType = "short"
	String DataType = "string"
)

type Endian string

const (
	LSB Endian = "LSB"
	MSB Endian = "MSB"
)

func GetMagickMime() ([]MIME, error) {
	var mime = MimeBase{
		Mimemap: Mimemap{
			MIME: make([]MIME, 0),
		},
	}

	err := xml.Unmarshal(data.MagickMimeXML, &mime.Mimemap)
	return mime.Mimemap.MIME, err
}
