package indexer

type Geometry struct {
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
}

type Size struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type Statistics struct {
	Min               float64 `json:"min,omitempty"`
	Max               float64 `json:"max,omitempty"`
	Mean              float64 `json:"mean,omitempty"`
	Median            float64 `json:"median,omitempty"`
	StandardDeviation float64 `json:"standardDeviation,omitempty"`
	Kurtosis          float64 `json:"kurtosis,omitempty"`
	Skewness          float64 `json:"skewness,omitempty"`
	Entropy           float64 `json:"entropy,omitempty"`
}

type Chromaticity struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type MagickImage struct {
	Name              string                   `json:"name,omitempty"`
	BaseName          string                   `json:"baseName,omitempty"`
	Permissions       int                      `json:"permissions,omitempty"`
	Format            string                   `json:"format,omitempty"`
	FormatDescription string                   `json:"formatDescription,omitempty"`
	MimeType          string                   `json:"mimeType,omitempty"`
	Class             string                   `json:"class,omitempty"`
	Geometry          *Geometry                `json:"geometry,omitempty"`
	Resolution        *Size                    `json:"resolution,omitempty"`
	PrintSize         *Size                    `json:"printSize,omitempty"`
	Units             string                   `json:"units,omitempty"`
	Type              string                   `json:"type,omitempty"`
	BaseType          string                   `json:"baseType,omitempty"`
	Endianness        string                   `json:"endianness,omitempty"`
	Colorspace        string                   `json:"colorspace,omitempty"`
	Depth             int                      `json:"depth,omitempty"`
	BaseDepth         int                      `json:"baseDepth,omitempty"`
	ChannelDepth      map[string]int           `json:"channelDepth,omitempty"`
	Pixels            int                      `json:"pixels,omitempty"`
	ImageStatistics   map[string]*Statistics   `json:"imageStatistics,omitempty"`
	ChannelStatistics map[string]*Statistics   `json:"channelStatistics,omitempty"`
	RenderingIntent   string                   `json:"renderingIntent,omitempty"`
	Gamma             float64                  `json:"gamma,omitempty"`
	Chromaticity      map[string]*Chromaticity `json:"chromaticity,omitempty"`
	MatteColor        string                   `json:"matteColor,omitempty"`
	BackgroundColor   string                   `json:"backgroundColor,omitempty"`
	BorderColor       string                   `json:"borderColor,omitempty"`
	TransparentColor  string                   `json:"transparentColor,omitempty"`
	Interlace         string                   `json:"interlace,omitempty"`
	Intensity         string                   `json:"intensity,omitempty"`
	Compose           string                   `json:"compose,omitempty"`
	PageGeometry      *Geometry                `json:"pageGeometry,omitempty"`
	Dispose           string                   `json:"dispose,omitempty"`
	Iterations        int                      `json:"iterations,omitempty"`
	Compression       string                   `json:"compression,omitempty"`
	Quality           float64                  `json:"quality,omitempty"`
	Orientation       string                   `json:"orientation,omitempty"`
	Properties        map[string]any           `json:"properties,omitempty"`
	Profiles          map[string]any           `json:"profiles,omitempty"`
	Tainted           bool                     `json:"tainted,omitempty"`
	Filesize          string                   `json:"filesize,omitempty"`
	NumberPixels      string                   `json:"numberPixels,omitempty"`
	PixelsPerSecond   string                   `json:"pixelsPerSecond,omitempty"`
	UserTime          string                   `json:"userTime,omitempty"`
	ElapsedTime       string                   `json:"elapsedTime,omitempty"`
	Version           string                   `json:"version,omitempty"`
}

type MagickResult struct {
	Version string       `json:"version"`
	Image   *MagickImage `json:"image"`
}
