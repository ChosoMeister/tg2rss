package rest

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

const gzipMinSize = 1024 // Minimum response size to compress

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
		return w
	},
}

// GzipMiddleware compresses responses using gzip when the client supports it
// and the response is large enough to benefit from compression.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip encoding
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(gz)

		gzw := &gzipResponseWriter{
			ResponseWriter: w,
			gzipWriter:     gz,
		}

		defer gzw.Close()

		next.ServeHTTP(gzw, r)
	})
}

// gzipResponseWriter wraps http.ResponseWriter with gzip compression.
// Compression is activated only after the buffered data exceeds gzipMinSize.
type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter  *gzip.Writer
	buf         []byte
	wroteHeader bool
	compressed  bool
	statusCode  int
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.wroteHeader = true
	// Don't write yet — defer until we know if we're compressing
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.compressed {
		return w.gzipWriter.Write(b)
	}

	w.buf = append(w.buf, b...)

	if len(w.buf) >= gzipMinSize {
		w.startCompression()
		return len(b), nil
	}

	return len(b), nil
}

func (w *gzipResponseWriter) startCompression() {
	w.compressed = true

	// Remove Content-Length since compressed size differs
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	w.ResponseWriter.Header().Add("Vary", "Accept-Encoding")

	if w.wroteHeader {
		w.ResponseWriter.WriteHeader(w.statusCode)
	}

	w.gzipWriter.Reset(w.ResponseWriter)

	if len(w.buf) > 0 {
		// nolint: errcheck
		w.gzipWriter.Write(w.buf)
		w.buf = nil
	}
}

// Close flushes any buffered data. If the response was too small for compression,
// write it uncompressed.
func (w *gzipResponseWriter) Close() {
	if w.compressed {
		// nolint: errcheck
		w.gzipWriter.Close()
		return
	}

	// Response was too small for compression — write uncompressed
	if w.wroteHeader {
		w.ResponseWriter.WriteHeader(w.statusCode)
	}

	if len(w.buf) > 0 {
		// nolint: errcheck
		w.ResponseWriter.Write(w.buf)
	}
}

// Unwrap returns the original ResponseWriter
func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Flush implements the http.Flusher interface
func (w *gzipResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		if w.compressed {
			// nolint: errcheck
			w.gzipWriter.Flush()
		}
		f.Flush()
	}
}
