package wallpaper

import (
	"bufio"
	"encoding/binary"
	"errors"
	"image"
	"io"
	"os"

	"github.com/disintegration/imaging"

	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper/internal/progjpeg"
)

// How much memory making the derivatives of one original takes, in MB per megapixel, measured
// with real downloads (A2 phase 5, docs/test-report-v1.0.md §13.5). A sequential JPEG costs
// about 1 + 1.5 x its samples per pixel (4:2:0 has 1.5, 4:4:4 has 3): 2.6-3.0 MB/MP for the
// Getty's 4:2:0 files, 5.0-5.7 for Commons' 4:4:4 ones. A progressive JPEG also keeps every
// coefficient of the image until its last scan: 4 bytes per sample in Go's image/jpeg (16-17
// MB/MP in all at 4:4:4), 2 bytes in progjpeg, which decodes progressive originals here.
const (
	costBaseMBPerMP        = 1.0
	costPerSampleMBPerMP   = 1.5
	costProgressivePerSamp = 2.0  // progjpeg keeps int16 coefficients
	costOtherMBPerMP       = 8.0  // PNG and anything else: 4-8 bytes per pixel decoded
	costUnknownMB          = 64.0 // a file whose header cannot be read
)

// jpegInfo is what a JPEG's frame header says about the cost of decoding it.
type jpegInfo struct {
	width, height int
	progressive   bool
	// samples is the number of samples per pixel over all components: 3 for 4:4:4,
	// 1.5 for 4:2:0, 1 for grey.
	samples float64
}

var errNotJPEG = errors.New("not a JPEG")

// readJPEGInfo reads the frame header (SOFn) of a JPEG stream.
func readJPEGInfo(r io.Reader) (jpegInfo, error) {
	br := bufio.NewReader(r)
	var soi [2]byte
	if _, err := io.ReadFull(br, soi[:]); err != nil || soi[0] != 0xFF || soi[1] != 0xD8 {
		return jpegInfo{}, errNotJPEG
	}
	for {
		// Find the next marker, skipping fill bytes.
		b, err := br.ReadByte()
		if err != nil {
			return jpegInfo{}, err
		}
		if b != 0xFF {
			continue
		}
		m, err := br.ReadByte()
		for err == nil && m == 0xFF {
			m, err = br.ReadByte()
		}
		if err != nil {
			return jpegInfo{}, err
		}
		if m == 0x01 || (m >= 0xD0 && m <= 0xD8) {
			continue // markers without a length
		}
		if m == 0xD9 || m == 0xDA {
			return jpegInfo{}, errors.New("no frame header before the image data")
		}
		var lenBuf [2]byte
		if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
			return jpegInfo{}, err
		}
		n := int(binary.BigEndian.Uint16(lenBuf[:])) - 2
		if n < 0 {
			return jpegInfo{}, errors.New("bad segment length")
		}
		isSOF := m >= 0xC0 && m <= 0xCF && m != 0xC4 && m != 0xC8 && m != 0xCC
		if !isSOF {
			if _, err := br.Discard(n); err != nil {
				return jpegInfo{}, err
			}
			continue
		}
		seg := make([]byte, n)
		if _, err := io.ReadFull(br, seg); err != nil {
			return jpegInfo{}, err
		}
		if len(seg) < 6 {
			return jpegInfo{}, errors.New("short frame header")
		}
		info := jpegInfo{
			height: int(binary.BigEndian.Uint16(seg[1:3])),
			width:  int(binary.BigEndian.Uint16(seg[3:5])),
			// SOF2, SOF6, SOF10 and SOF14 are progressive.
			progressive: m == 0xC2 || m == 0xC6 || m == 0xCA || m == 0xCE,
		}
		nc := int(seg[5])
		if nc == 0 || len(seg) < 6+3*nc {
			return jpegInfo{}, errors.New("short frame header")
		}
		hMax, vMax := 1, 1
		for i := 0; i < nc; i++ {
			s := seg[6+3*i+1]
			hMax = max(hMax, int(s>>4))
			vMax = max(vMax, int(s&0x0F))
		}
		for i := 0; i < nc; i++ {
			s := seg[6+3*i+1]
			info.samples += float64(int(s>>4)*int(s&0x0F)) / float64(hMax*vMax)
		}
		return info, nil
	}
}

// costMBPerMP is the memory, in MB per megapixel, that making the derivatives of this JPEG takes.
func (j jpegInfo) costMBPerMP() float64 {
	c := costBaseMBPerMP + costPerSampleMBPerMP*j.samples
	if j.progressive {
		c += costProgressivePerSamp * j.samples
	}
	return c
}

// masterDecodeCostMB estimates the memory, in MB, that decoding and fitting the original at path
// takes: from the JPEG frame header (size, progressive or not, chroma subsampling), else from the
// image size.
func masterDecodeCostMB(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return int64(costUnknownMB)
	}
	defer f.Close()
	if info, err := readJPEGInfo(f); err == nil {
		return roundUpMB(float64(info.width) * float64(info.height) / 1e6 * info.costMBPerMP())
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return int64(costUnknownMB)
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return int64(costUnknownMB)
	}
	return roundUpMB(float64(cfg.Width) * float64(cfg.Height) / 1e6 * costOtherMBPerMP)
}

func roundUpMB(mb float64) int64 {
	n := int64(mb + 0.999)
	if n < 1 {
		n = 1
	}
	return n
}

// isProgressiveJPEG reports whether the file at path is a progressive JPEG.
func isProgressiveJPEG(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := readJPEGInfo(f)
	return err == nil && info.progressive
}

// openMaster decodes an original for making derivatives. A progressive JPEG goes through
// progjpeg, which needs half the memory of image/jpeg for it and gives the same image; anything
// else through imaging.Open. Like imaging.Open without options, EXIF orientation is not applied.
func openMaster(path string) (image.Image, error) {
	if !isProgressiveJPEG(path) {
		return imaging.Open(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return progjpeg.Decode(bufio.NewReader(f))
}
