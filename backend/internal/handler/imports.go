package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"invoicekeeper/internal/middleware"
	"invoicekeeper/internal/service"
)

// importStatement — POST /imports/statement (multipart: file). Находит регулярные списания, ничего не сохраняет.
func (h *handlers) importStatement(c *gin.Context) {
	if h.Imports == nil {
		fail(c, service.Forbidden("import is not available"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxStatementSize+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		fail(c, service.Invalid("file is required"))
		return
	}
	if fh.Size > service.MaxStatementSize {
		fail(c, service.Invalid("statement is too large: max 5 MB"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, service.Invalid("file is required"))
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, service.MaxStatementSize+1))
	if err != nil {
		fail(c, service.Invalid("file is required"))
		return
	}
	res, err := h.Imports.Analyze(c.Request.Context(), middleware.ActorOf(c), fh.Filename, data)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// importConfirm — POST /imports/confirm {file, items:[{title, amount, currency, period, due_date}]}.
func (h *handlers) importConfirm(c *gin.Context) {
	if h.Imports == nil {
		fail(c, service.Forbidden("import is not available"))
		return
	}
	var in struct {
		File  string               `json:"file"`
		Items []service.ImportItem `json:"items"`
	}
	if !bind(c, &in) {
		return
	}
	created, failed, err := h.Imports.Confirm(c.Request.Context(), middleware.ActorOf(c), in.File, in.Items)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"created": created, "failed": nonNil(failed)})
}
