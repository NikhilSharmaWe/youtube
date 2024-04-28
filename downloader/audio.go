package downloader

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/NikhilSharmaWe/youtube"
	"golang.org/x/net/http/httpproxy"
)

var (
	logLevel   = "info"
	downloader *Downloader
)

func (dl *Downloader) DownloadAudio(ctx context.Context, outputPath string, v *youtube.Video, mimetype, language string) error {
	audioFormat, err1 := getAudioFormat(v, mimetype, language)
	if err1 != nil {
		return err1
	}

	log := youtube.Logger.With("id", v.ID)

	log.Info(
		"Downloading audio",
		"audioMimeType", audioFormat.MimeType,
	)

	err := os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err != nil {
		return err
	}

	audioFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}

	err = dl.videoDLWorker(ctx, audioFile, v, audioFormat)
	if err != nil {
		return err
	}

	return nil
}

func GetDownloader(outputDir string) *Downloader {
	if downloader != nil {
		return downloader
	}

	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	httpTransport := &http.Transport{
		// Proxy: http.ProxyFromEnvironment() does not work. Why?
		Proxy: func(r *http.Request) (uri *url.URL, err error) {
			return proxyFunc(r.URL)
		},
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	youtube.SetLogLevel(logLevel)

	downloader = &Downloader{
		OutputDir: outputDir,
	}
	downloader.HTTPClient = &http.Client{Transport: httpTransport}

	return downloader
}

func (dl *Downloader) GetVideoWithFormat(videoID string, outputDir string) (*youtube.Video, *youtube.Format, error) {
	video, err := dl.GetVideo(videoID)
	if err != nil {
		return nil, nil, err
	}

	itag, _ := strconv.Atoi("medium")
	formats := video.Formats

	if itag > 0 {
		formats = formats.Itag(itag)
	}
	if formats == nil {
		return nil, nil, fmt.Errorf("unable to find the specified format")
	}

	formats.Sort()

	// select the first format
	return video, &formats[0], nil
}

func getAudioFormat(v *youtube.Video, mimetype, language string) (*youtube.Format, error) {
	var audioFormats youtube.FormatList

	formats := v.Formats
	if mimetype != "" {
		formats = formats.Type(mimetype)
	}

	audioFormats = formats.Type("audio")

	if language != "" {
		audioFormats = audioFormats.Language(language)
	}

	if len(audioFormats) == 0 {
		return nil, errors.New("no audio format found after filtering")
	}

	audioFormats.Sort()

	return &audioFormats[0], nil
}
