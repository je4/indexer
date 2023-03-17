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
	"bytes"
	"context"
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	ffmpeg_models "github.com/je4/goffmpeg/models"
	"golang.org/x/exp/slices"
	"io"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

type FFMPEGMime struct {
	Video  bool
	Audio  bool
	Format string
	Mime   string
}

var regexpFFProbeDuration = regexp.MustCompile("^([0-9]+):([0-9]+):([0-9]+).([0-9]{2})$")
var regexFFProbeMime = regexp.MustCompile("^((audio|video)/.*)|(application/mp4)|(application/mpeg)$")

func parseDuration(t string) (time.Duration, error) {

	matches := regexpFFProbeDuration.FindStringSubmatch(t)
	if matches == nil {
		return 0, errors.New(fmt.Sprintf("cannot convert %s to duration", t))
	}

	hours, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, errors.Wrapf(err, "invalid hours %s in %s", matches[1], t)
	}
	mins, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, errors.Wrapf(err, "invalid min %s in %s", matches[2], t)
	}
	secs, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, errors.Wrapf(err, "invalid sec %s in %s", matches[3], t)
	}

	hundreds, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, errors.Wrapf(err, "invalid sec %s in %s", matches[3], t)
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(mins)*time.Minute +
		time.Duration(secs)*time.Second +
		time.Duration(hundreds*10)*time.Millisecond, nil
}

type ActionFFProbe struct {
	name    string
	ffprobe string
	wsl     bool
	timeout time.Duration
	caps    ActionCapability
	server  *Server
	mime    []FFMPEGMime
}

func NewActionFFProbe(name string, ffprobe string, wsl bool, timeout time.Duration, online bool, mime []FFMPEGMime, server *Server, ad *ActionDispatcher) Action {
	var caps ActionCapability = ACTFILEHEAD | ACTSTREAM
	if online {
		caps |= ACTALLPROTO
	}
	af := &ActionFFProbe{name: name, ffprobe: ffprobe, wsl: wsl, timeout: timeout, caps: caps, server: server, mime: mime}
	ad.RegisterAction(af)
	return af
}

func (as *ActionFFProbe) GetWeight() uint {
	return 50
}

func (as *ActionFFProbe) GetCaps() ActionCapability {
	return as.caps
}

func (as *ActionFFProbe) GetName() string {
	return as.name
}

func (as *ActionFFProbe) Stream(contentType string, reader io.Reader, filename string) (*ResultV2, error) {
	if slices.Contains([]string{"image", "pdf"}, contentType) {
		return nil, nil
	}
	cmdparam := []string{"-i", "-", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", "-show_error"}
	cmdfile := as.ffprobe
	if as.wsl {
		cmdparam = append([]string{cmdfile}, cmdparam...)
		cmdfile = "wsl"
	}

	var out bytes.Buffer
	out.Grow(1024 * 1024) // 1MB size
	ctx, cancel := context.WithTimeout(context.Background(), as.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdfile, cmdparam...)
	cmd.Stdin = reader
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "error executing (%s %s) for file '%s': %v", cmdfile, cmdparam, filename, out.String())
	}

	var metadata ffmpeg_models.Metadata
	if err := json.Unmarshal([]byte(out.String()), &metadata); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshall metadata: %s", out.String())
	}

	// calculate duration and dimension
	var result = NewResultV2()
	d, _ := strconv.ParseFloat(metadata.Format.Duration, 64)
	result.Duration = uint(d) // time.Duration(d * float64(time.Second))
	var hasAudio, hasVideo bool
	for _, stream := range metadata.Streams {
		if stream.Width > 0 || stream.Height > 0 {
			result.Width = uint(stream.Width)
			result.Height = uint(stream.Height)
		}
		if stream.CodecType == "audio" {
			hasAudio = true
		}
		if stream.CodecType == "video" {
			hasVideo = true
		}
		if stream.CodecType == "data" {
			//hasData = true
		}
	}

	for _, m := range as.mime {
		if m.Audio == hasAudio && m.Video == hasVideo && m.Format == metadata.Format.FormatName {
			result.Mimetypes = append(result.Mimetypes, m.Mime)
		}
	}
	result.Metadata[as.GetName()] = metadata
	return result, nil
}

func (as *ActionFFProbe) Do(uri *url.URL, contentType string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, []string, []string, error) {
	var metadata ffmpeg_models.Metadata
	var filename string
	var err error

	if !regexFFProbeMime.MatchString(contentType) {
		return nil, nil, nil, ErrMimeNotApplicable
	}

	// local files need some adjustments...
	if uri.Scheme == "file" {
		filename, err = as.server.fm.Get(uri)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "invalid file uri %s", uri.String())
		}
		if as.wsl {
			filename = pathToWSL(filename)
		}
	} else {
		filename = uri.String()
	}

	cmdparam := []string{"-i", filename, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", "-show_error"}
	cmdfile := as.ffprobe
	if as.wsl {
		cmdparam = append([]string{cmdfile}, cmdparam...)
		cmdfile = "wsl"
	}

	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), as.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cmdfile, cmdparam...)
	cmd.Stdout = &out

	err = cmd.Run()
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "error executing (%s %s): %v", cmdfile, cmdparam, out.String())
	}

	if err = json.Unmarshal([]byte(out.String()), &metadata); err != nil {
		return nil, nil, nil, errors.Wrapf(err, "cannot unmarshall metadata: %s", out.String())
	}

	// calculate duration and dimension
	d, _ := strconv.ParseFloat(metadata.Format.Duration, 64)
	*duration = time.Duration(d * float64(time.Second))
	var hasAudio, hasVideo bool
	for _, stream := range metadata.Streams {
		if stream.Width > 0 || stream.Height > 0 {
			*width = uint(stream.Width)
			*height = uint(stream.Height)
		}
		if stream.CodecType == "audio" {
			hasAudio = true
		}
		if stream.CodecType == "video" {
			hasVideo = true
		}
		if stream.CodecType == "data" {
			//hasData = true
		}
	}

	mimetypes := []string{}
	for _, m := range as.mime {
		if m.Audio == hasAudio && m.Video == hasVideo && m.Format == metadata.Format.FormatName {
			mimetypes = append(mimetypes, m.Mime)
		}
	}
	return metadata, mimetypes, nil, nil
}

var (
	_ Action = &ActionFFProbe{}
)
