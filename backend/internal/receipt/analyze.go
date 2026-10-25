package receipt

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg" // декодеры для QR
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Config — где искать утилиты распознавания. Пустой путь — искать в PATH.
type Config struct {
	AI       AIConfig
	OCRLangs string // языки Tesseract, по умолчанию rus+eng
	Loc      *time.Location
}

// Analyzer — распознавание чека: QR → текст PDF → OCR (нейросеть Tesseract LSTM) → ИИ-модель (если задан ключ).
type Analyzer struct {
	cfg       Config
	tesseract string
	pdftotext string
	pdftoppm  string
	ai        *aiClient
}

func New(cfg Config) *Analyzer {
	if cfg.OCRLangs == "" {
		cfg.OCRLangs = "rus+eng"
	}
	if cfg.Loc == nil {
		cfg.Loc = time.Local
	}
	a := &Analyzer{cfg: cfg, ai: newAI(cfg.AI)}
	a.tesseract, _ = exec.LookPath("tesseract")
	a.pdftotext, _ = exec.LookPath("pdftotext")
	a.pdftoppm, _ = exec.LookPath("pdftoppm")
	return a
}

// Capabilities — что доступно (показывается администратору в настройках).
type Capabilities struct {
	QR  bool   `json:"qr"`
	PDF bool   `json:"pdf"`
	OCR bool   `json:"ocr"`
	AI  string `json:"ai"` // «provider · model» или пусто
}

func (a *Analyzer) Capabilities() Capabilities {
	c := Capabilities{QR: true, PDF: a.pdftotext != "", OCR: a.tesseract != ""}
	if a.ai != nil {
		c.AI = a.ai.cfg.Label()
	}
	return c
}

// Analyze распознаёт документ. Ошибки отдельных шагов не прерывают анализ — пишутся в Errors.
func (a *Analyzer) Analyze(ctx context.Context, data []byte, mime string) *Extracted {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	e := &Extracted{}
	dir, err := os.MkdirTemp("", "receipt-*")
	if err != nil {
		e.Errors = append(e.Errors, "tmp: "+err.Error())
		return e
	}
	defer os.RemoveAll(dir)

	var pagePNG []byte // первая страница PDF картинкой (QR, OCR, ИИ)
	switch mime {
	case "application/pdf":
		src := filepath.Join(dir, "doc.pdf")
		_ = os.WriteFile(src, data, 0o600)
		if a.pdftotext != "" {
			out, err := a.run(ctx, a.pdftotext, "-layout", "-l", "3", src, "-")
			if err != nil {
				e.Errors = append(e.Errors, "pdftotext: "+err.Error())
			} else if strings.TrimSpace(out) != "" {
				e.addMethod("pdf")
				ParseText(e, out, a.cfg.Loc)
			}
		}
		if a.pdftoppm != "" {
			if _, err := a.run(ctx, a.pdftoppm, "-r", "200", "-png", "-f", "1", "-l", "1", "-singlefile", src, filepath.Join(dir, "page")); err != nil {
				e.Errors = append(e.Errors, "pdftoppm: "+err.Error())
			} else {
				pagePNG, _ = os.ReadFile(filepath.Join(dir, "page.png"))
			}
		}
		if pagePNG != nil {
			a.scanQR(e, pagePNG)
			// скан без текстового слоя — распознаём картинку
			if len(strings.TrimSpace(e.Text)) < 20 {
				a.ocr(ctx, e, filepath.Join(dir, "page.png"))
			}
		}
	default: // изображение
		a.scanQR(e, data)
		ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[mime]
		src := filepath.Join(dir, "img"+ext)
		_ = os.WriteFile(src, data, 0o600)
		a.ocr(ctx, e, src)
	}

	if a.ai != nil {
		r, err := a.ai.check(ctx, data, mime, pagePNG)
		if err != nil {
			e.Errors = append(e.Errors, err.Error())
		} else {
			e.addMethod("ai")
			a.mergeAI(e, r)
		}
	}
	if len(e.Text) > 20000 {
		e.Text = e.Text[:20000]
	}
	return e
}

func (a *Analyzer) scanQR(e *Extracted, imgData []byte) {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return // webp и прочее — QR не ищем, OCR/ИИ справятся
	}
	raw, ok := DecodeQR(img)
	if !ok {
		return
	}
	f, ok := ParseFiscal(raw, a.cfg.Loc)
	if !ok {
		e.Text += "\nQR: " + raw
		return
	}
	e.addMethod("qr")
	e.Fiscal = f
	e.PaymentLike = true
	sum := f.Sum
	e.Total, e.TotalSource = &sum, "qr"
	e.Amounts = appendDec(e.Amounts, sum)
	d := time.Date(f.Time.Year(), f.Time.Month(), f.Time.Day(), 0, 0, 0, 0, a.cfg.Loc)
	e.Date = &d
	e.OperationID = f.OperationID()
	if e.Currency == "" {
		e.Currency = "RUB" // кассовые чеки ФНС — всегда в рублях
	}
	if f.Kind == "2" || f.Kind == "4" {
		e.Negative = appendUniq(e.Negative, "чек возврата")
	}
}

func (a *Analyzer) ocr(ctx context.Context, e *Extracted, path string) {
	if a.tesseract == "" {
		return
	}
	out, err := a.run(ctx, a.tesseract, path, "stdout", "-l", a.cfg.OCRLangs, "--psm", "3")
	if err != nil {
		// нет русского словаря — пробуем английский, чтобы хотя бы цифры распознались
		out, err = a.run(ctx, a.tesseract, path, "stdout", "--psm", "3")
	}
	if err != nil {
		e.Errors = append(e.Errors, "ocr: "+err.Error())
		return
	}
	if strings.TrimSpace(out) != "" {
		e.addMethod("ocr")
		ParseText(e, out, a.cfg.Loc)
	}
}

// mergeAI: QR-код кассового чека надёжнее модели, поэтому его данные не перезаписываются.
func (a *Analyzer) mergeAI(e *Extracted, r *AIResult) {
	e.AI = r
	if r.IsPaymentDocument {
		e.PaymentLike = true
	}
	if r.Success != nil && !*r.Success {
		e.Negative = appendUniq(e.Negative, "ИИ: операция не выполнена")
	}
	if e.Fiscal != nil {
		return
	}
	if r.Amount != nil {
		if d, err := decimal.NewFromString(strings.ReplaceAll(strings.ReplaceAll(*r.Amount, " ", ""), ",", ".")); err == nil && d.Sign() > 0 {
			e.Total, e.TotalSource = &d, "ai"
			e.Amounts = appendDec(e.Amounts, d)
		}
	}
	if r.Currency != nil && *r.Currency != "" {
		e.Currency = strings.ToUpper(*r.Currency)
	}
	if r.Date != nil {
		if t, err := time.ParseInLocation("2006-01-02", *r.Date, a.cfg.Loc); err == nil {
			e.Date = &t
		}
	}
	if r.OperationID != nil && *r.OperationID != "" && e.OperationID == "" {
		e.OperationID = *r.OperationID
	}
	if r.Payee != nil && *r.Payee != "" && e.Payee == "" {
		e.Payee = *r.Payee
	}
}

func (a *Analyzer) run(ctx context.Context, bin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return "", &runError{err: err, msg: msg}
	}
	return out.String(), nil
}

type runError struct {
	err error
	msg string
}

func (e *runError) Error() string {
	if e.msg != "" {
		return e.err.Error() + ": " + e.msg
	}
	return e.err.Error()
}

// ErrNoPDFText — на сервере нет pdftotext (poppler-utils): текст из PDF не извлечь.
var ErrNoPDFText = errors.New("pdftotext is not installed")

// PDFText — весь текст PDF с сохранением колонок (для банковских выписок, до 30 страниц).
func (a *Analyzer) PDFText(ctx context.Context, data []byte) (string, error) {
	if a.pdftotext == "" {
		return "", ErrNoPDFText
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "statement-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "statement.pdf")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		return "", err
	}
	return a.run(ctx, a.pdftotext, "-layout", "-l", "30", "-enc", "UTF-8", src, "-")
}
