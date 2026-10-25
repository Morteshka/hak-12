package receipt

import (
	"image"
	"image/color"
	"net/url"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/shopspring/decimal"
)

// Fiscal — данные QR-кода кассового чека по формату ФНС: t=20261024T1530&s=399.00&fn=…&i=…&fp=…&n=1.
type Fiscal struct {
	Raw  string          `json:"raw"`
	Time time.Time       `json:"time"`
	Sum  decimal.Decimal `json:"sum"`
	FN   string          `json:"fn"` // номер фискального накопителя
	FD   string          `json:"fd"` // номер фискального документа
	FP   string          `json:"fp"` // фискальный признак
	Kind string          `json:"n"`  // 1 — приход (оплата), 2 — возврат прихода, 3 — расход, 4 — возврат расхода
}

// ParseFiscal разбирает строку QR кассового чека. ok=false — это не фискальный QR.
func ParseFiscal(raw string, loc *time.Location) (*Fiscal, bool) {
	q, err := url.ParseQuery(strings.TrimSpace(raw))
	if err != nil || q.Get("t") == "" || q.Get("s") == "" || q.Get("fn") == "" {
		return nil, false
	}
	sum, err := decimal.NewFromString(strings.ReplaceAll(q.Get("s"), ",", "."))
	if err != nil {
		return nil, false
	}
	var ts time.Time
	for _, layout := range []string{"20060102T150405", "20060102T1504"} {
		if ts, err = time.ParseInLocation(layout, q.Get("t"), loc); err == nil {
			break
		}
	}
	if err != nil {
		return nil, false
	}
	return &Fiscal{Raw: raw, Time: ts, Sum: sum, FN: q.Get("fn"), FD: q.Get("i"), FP: q.Get("fp"), Kind: q.Get("n")}, true
}

// OperationID — уникальный номер чека: по нему ловим повторное использование.
func (f *Fiscal) OperationID() string { return "ФН " + f.FN + " ФД " + f.FD + " ФП " + f.FP }

// KindRU — тип расчёта по-русски.
func (f *Fiscal) KindRU() string {
	switch f.Kind {
	case "1", "":
		return "приход"
	case "2":
		return "возврат прихода"
	case "3":
		return "расход"
	case "4":
		return "возврат расхода"
	}
	return "тип " + f.Kind
}

// DecodeQR ищет QR-код на изображении: оригинал, затем уменьшенная и контрастная копии
// (фото чека часто крупные и с бликами).
func DecodeQR(img image.Image) (string, bool) {
	reader := qrcode.NewQRCodeReader()
	hints := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
	try := func(im image.Image) (string, bool) {
		bmp, err := gozxing.NewBinaryBitmapFromImage(im)
		if err != nil {
			return "", false
		}
		r, err := reader.Decode(bmp, hints)
		if err != nil {
			return "", false
		}
		return r.GetText(), true
	}
	if s, ok := try(img); ok {
		return s, true
	}
	b := img.Bounds()
	if w := b.Dx(); w > 1400 {
		if s, ok := try(scale(img, 1400.0/float64(w))); ok {
			return s, true
		}
	}
	if s, ok := try(contrast(img)); ok {
		return s, true
	}
	return "", false
}

// scale — простое уменьшение (ближайший сосед) без внешних библиотек.
func scale(src image.Image, k float64) image.Image {
	b := src.Bounds()
	w, h := int(float64(b.Dx())*k), int(float64(b.Dy())*k)
	dst := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(b.Min.X+int(float64(x)/k), b.Min.Y+int(float64(y)/k)))
		}
	}
	return dst
}

// contrast — перевод в ч/б по порогу: помогает с серыми фото.
func contrast(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g := color.GrayModel.Convert(src.At(x, y)).(color.Gray)
			if g.Y > 140 {
				g.Y = 255
			} else {
				g.Y = 0
			}
			dst.SetGray(x, y, g)
		}
	}
	return dst
}
