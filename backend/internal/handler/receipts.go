package handler

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
	"invoicekeeper/internal/service"
)

// uploadReceipt — POST /payment-occurrences/:id/receipts (multipart/form-data: file, note).
func (h *handlers) uploadReceipt(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxReceiptSize+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required (multipart field \"file\", up to 10 MB)"})
		return
	}
	if fh.Size > service.MaxReceiptSize {
		fail(c, service.Invalid("file is too large: max 10 MB"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, service.MaxReceiptSize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}
	res, err := h.Receipts.Submit(c.Request.Context(), middleware.ActorOf(c), id,
		service.Upload{Name: fh.Filename, Note: c.PostForm("note"), Source: "web", Data: data})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *handlers) occurrenceReceipts(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	a := middleware.ActorOf(c)
	if _, err := h.Payments.GetOccurrence(c.Request.Context(), a, id); err != nil {
		fail(c, err)
		return
	}
	list, err := h.Receipts.List(c.Request.Context(), a, "", id, 50)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(list))
}

func (h *handlers) listReceipts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	list, err := h.Receipts.List(c.Request.Context(), middleware.ActorOf(c), c.Query("status"), 0, limit)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(list))
}

func (h *handlers) getReceipt(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	r, err := h.Receipts.Get(c.Request.Context(), middleware.ActorOf(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

// receiptFile отдаёт сам файл. Заголовки не дают браузеру выполнить содержимое как страницу.
func (h *handlers) receiptFile(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	r, data, err := h.Receipts.File(c.Request.Context(), middleware.ActorOf(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	disp := "inline"
	if c.Query("download") == "1" {
		disp = "attachment"
	}
	c.Header("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": r.FileName}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; sandbox")
	c.Header("Cache-Control", "private, max-age=300")
	c.Data(http.StatusOK, r.Mime, data)
}

func (h *handlers) receiptHistory(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	list, err := h.Receipts.History(c.Request.Context(), middleware.ActorOf(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nonNil(list))
}

type decisionInput struct {
	PaidAt  *string `json:"paid_at"`
	Comment string  `json:"comment"`
}

func (h *handlers) approveReceipt(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in decisionInput
	if c.Request.ContentLength > 0 && !bind(c, &in) {
		return
	}
	res, err := h.Receipts.Approve(c.Request.Context(), middleware.ActorOf(c), id, in.PaidAt, in.Comment)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *handlers) rejectReceipt(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in decisionInput
	if !bind(c, &in) {
		return
	}
	res, err := h.Receipts.Reject(c.Request.Context(), middleware.ActorOf(c), id, in.Comment)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *handlers) recheckReceipt(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	res, err := h.Receipts.Recheck(c.Request.Context(), middleware.ActorOf(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
