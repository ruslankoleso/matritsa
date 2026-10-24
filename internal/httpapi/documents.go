package httpapi

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"matritsa/internal/httpapi/gen"
	"matritsa/internal/products"
	"matritsa/internal/store/db"
	"matritsa/internal/workflow"
)

const (
	maxUpload    = 5 << 20
	maxDocuments = 10 // per application: a client needs two or three, the cap only protects the disk
)

var allowedTypes = map[string]string{"application/pdf": ".pdf", "image/png": ".png", "image/jpeg": ".jpg"}

func safeName(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' || r == ' ':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 80 {
		out = out[len(out)-80:]
	}
	if out == "" {
		out = "document"
	}
	return out
}

func (s *Server) UploadDocument(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	app, _, ok := s.own(w, r, id.String())
	if !ok {
		return
	}
	if st := workflow.Status(app.Status); st != workflow.Draft && st != workflow.NeedsInfo {
		writeError(w, http.StatusConflict, "invalid_state", "Сейчас документы добавлять не нужно")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", "Файл слишком большой — не больше 5 МБ")
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "Не удалось прочитать файл")
		return
	}
	kind := r.FormValue("kind")
	prod, _ := products.Get(products.ID(app.ProductID))
	known := false
	for _, d := range prod.Documents {
		known = known || d.Kind == kind
	}
	if !known {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Неизвестный тип документа", []fieldErr{{"kind", "Выберите тип документа из списка для этого продукта"}})
		return
	}
	if have, err := s.Store.ListDocuments(r.Context(), app.ID); err != nil {
		internalError(w, r, err)
		return
	} else if len(have) >= maxDocuments {
		writeError(w, http.StatusUnprocessableEntity, "too_many_files", "К заявке уже приложено много файлов. Удалите лишние или оставьте самые нужные")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Приложите файл", []fieldErr{{"file", "Выберите файл"}})
		return
	}
	defer file.Close()
	if header.Size > maxUpload {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "Файл слишком большой — не больше 5 МБ")
		return
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	ctype := http.DetectContentType(head[:n])
	ext, allowed := allowedTypes[ctype]
	if !allowed {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_type", "Подходят файлы PDF, PNG и JPEG")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		internalError(w, r, err)
		return
	}
	rel := filepath.Join("uploads", app.ID, uuid.NewString()+ext)
	full := filepath.Join(s.DataDir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		internalError(w, r, err)
		return
	}
	dst, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		internalError(w, r, err)
		return
	}
	size, err := io.Copy(dst, file)
	_ = dst.Close()
	if err != nil {
		_ = os.Remove(full)
		internalError(w, r, err)
		return
	}
	doc, err := s.Store.InsertDocument(r.Context(), db.InsertDocumentParams{
		ApplicationID: app.ID, Kind: kind, Filename: safeName(header.Filename), ContentType: ctype, Size: size, Path: rel,
	})
	if err != nil {
		_ = os.Remove(full)
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, gen.Document{Id: uid(doc.ID), Kind: doc.Kind, Filename: doc.Filename, Size: doc.Size, UploadedAt: doc.UploadedAt.Time})
}
