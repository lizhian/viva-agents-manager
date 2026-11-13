package agentsmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
)

var imagePattern = regexp.MustCompile(`!\[[^\]]*\]\((https?://[^\s)]+)\)`)

type imageState struct {
	source  string
	token   uint64
	cancel  context.CancelFunc
	bitmaps map[string]gfx.Bitmap
	changed bool
}
type imageResult struct {
	view        *editorView
	token       uint64
	source, url string
	bitmap      gfx.Bitmap
	err         error
}

func (s *imageState) clear() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.token++
	s.bitmaps = nil
	s.source = ""
	s.changed = true
}
func (s *imageState) lookup(url string) (gfx.Bitmap, bool) { b, ok := s.bitmaps[url]; return b, ok }
func (v *editorView) loadImages() {
	d := v.p.buffer(v.selected)
	if d == nil {
		return
	}
	v.images.clear()
	source := d.body.String()
	v.images.source = source
	token := v.images.token
	matches := imagePattern.FindAllStringSubmatch(source, 8)
	if len(matches) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	v.images.cancel = cancel
	go func() {
		defer cancel()
		for _, match := range matches {
			bitmap, err := fetchImage(ctx, match[1])
			if ctx.Err() != nil {
				return
			}
			p := v.p
			p.imageMu.Lock()
			p.imageResults = append(p.imageResults, imageResult{view: v, token: token, source: source, url: match[1], bitmap: bitmap, err: err})
			p.imageMu.Unlock()
			if p.opts.Invalidator != nil {
				p.opts.Invalidator.Invalidate()
			}
		}
	}()
}
func (p *Page) Wake() {
	p.imageMu.Lock()
	results := p.imageResults
	p.imageResults = nil
	p.imageMu.Unlock()
	for _, r := range results {
		v := r.view
		d := p.buffer(v.selected)
		if d == nil || v.images.token != r.token || d.body.String() != r.source {
			continue
		}
		if r.err != nil {
			p.result(r.err)
			continue
		}
		if v.images.bitmaps == nil {
			v.images.bitmaps = map[string]gfx.Bitmap{}
		}
		v.images.bitmaps[r.url] = r.bitmap
		v.images.changed = true
	}
	if len(results) > 0 {
		p.invalidate()
	}
}
func fetchImage(ctx context.Context, url string) (gfx.Bitmap, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return gfx.Bitmap{}, fmt.Errorf("不支持的图片地址")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return gfx.Bitmap{}, err
	}
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return gfx.Bitmap{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return gfx.Bitmap{}, fmt.Errorf("图片 HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil {
		return gfx.Bitmap{}, err
	}
	if len(data) > 8<<20 {
		return gfx.Bitmap{}, fmt.Errorf("图片超过 8 MiB")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return gfx.Bitmap{}, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16<<20 {
		return gfx.Bitmap{}, fmt.Errorf("图片尺寸过大")
	}
	im, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return gfx.Bitmap{}, err
	}
	rgba := image.NewRGBA(image.Rect(0, 0, config.Width, config.Height))
	draw.Draw(rgba, rgba.Bounds(), im, im.Bounds().Min, draw.Src)
	hash := sha256.Sum256(data)
	return gfx.Bitmap{Pix: rgba.Pix, Width: config.Width, Height: config.Height, Stride: rgba.Stride, ID: binary.LittleEndian.Uint64(hash[:8]), Version: 1}, nil
}
