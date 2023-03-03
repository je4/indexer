package data

import _ "embed"

// MagickMimeXML https://github.com/ImageMagick/ImageMagick/blob/main/config/mime.xml
//
//go:embed mime.xml
var MagickMimeXML []byte
