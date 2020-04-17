package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/goph/emperror"
	ffmpeg_models "github.com/je4/goffmpeg/models"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

var regexpFFProbeDuration = regexp.MustCompile("^([0-9]+):([0-9]+):([0-9]+).([0-9]{2})$")
var regexFFProbeMime = regexp.MustCompile("^(audio|video)/.*$")

func parseDuration(t string) (time.Duration, error) {

	matches := regexpFFProbeDuration.FindStringSubmatch(t)
	if matches == nil {
		return 0, errors.New(fmt.Sprintf("cannot convert %s to duration", t))
	}

	hours, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, emperror.Wrapf(err, "invalid hours %s in %s", matches[1], t)
	}
	mins, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, emperror.Wrapf(err, "invalid min %s in %s", matches[2], t)
	}
	secs, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, emperror.Wrapf(err, "invalid sec %s in %s", matches[3], t)
	}

	hundreds, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, emperror.Wrapf(err, "invalid sec %s in %s", matches[3], t)
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
}

func NewActionFFProbe(ffprobe string, wsl bool, timeout time.Duration) Action {
	return &ActionFFProbe{name: "ffprobe", ffprobe: ffprobe, wsl: wsl, timeout: timeout}
}

func (as *ActionFFProbe) GetCaps() ActionCapability {
	return ACTALLPROTO
}

func (as *ActionFFProbe) GetName() string {
	return as.name
}

func (as *ActionFFProbe) Do(uri *url.URL, mimetype *string, width *uint, height *uint, duration *time.Duration) (interface{}, error) {
	var metadata ffmpeg_models.Metadata
	var filename string
	var err error

	if !regexFFProbeMime.MatchString(*mimetype) {
		return nil, ErrMimeNotApplicable
	}

	// local files need some adjustments...
	if uri.Scheme == "file" {
		filename, err = getFilePath(uri)
		if err != nil {
			return nil, emperror.Wrapf(err, "invalid file uri %s", uri.String())
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
		return nil, emperror.Wrapf(err, "error executing (%s %s): %v", cmdfile, cmdparam, out.String())
	}

	if err = json.Unmarshal([]byte(out.String()), &metadata); err != nil {
		return nil, emperror.Wrapf(err, "cannot unmarshall metadata: %s", out.String())
	}

	// calculate duration and dimension
	d, _ := strconv.ParseFloat(metadata.Format.Duration, 64)
	*duration = time.Duration(d * float64(time.Second))
	for _, stream := range metadata.Streams {
		if stream.Width > 0 || stream.Height > 0 {
			*width = uint(stream.Width)
			*height = uint(stream.Height)
		}
	}

	return metadata, nil
}
