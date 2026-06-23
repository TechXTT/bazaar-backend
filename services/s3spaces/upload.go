package s3spaces

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"

	"github.com/gofrs/uuid/v5"
)

// allowedImageTypes maps the content types we accept (as reported by
// http.DetectContentType) to the canonical file extension we store them under.
// Generating the extension server-side from sniffed bytes — rather than
// trusting the client filename — closes the path-traversal / stored-XSS vector
// in BE-5.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// detectImageExt sniffs the first 512 bytes of file, validates the content type
// against the image allowlist, and returns the canonical extension. The file
// read offset is reset to the start so callers can persist the full content.
func detectImageExt(file multipart.File) (string, error) {
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	contentType := http.DetectContentType(head[:n])
	// DetectContentType may append parameters (e.g. "; charset=..."); strip them.
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	ext, ok := allowedImageTypes[contentType]
	if !ok {
		return "", fmt.Errorf("unsupported file type %q; only images are allowed", contentType)
	}
	return ext, nil
}

// buildObjectKey produces a sanitized, server-controlled storage key of the form
// <cleanPrefix>/<uuid><ext>. The client never influences the filename, and the
// prefix is cleaned and checked for traversal so it cannot escape its namespace.
func buildObjectKey(keyPrefix, ext string) (string, error) {
	cleanPrefix := path.Clean("/" + strings.TrimSpace(keyPrefix))
	cleanPrefix = strings.Trim(cleanPrefix, "/")
	if cleanPrefix == "" || cleanPrefix == "." {
		return "", fmt.Errorf("invalid upload key prefix")
	}
	if strings.Contains(cleanPrefix, "..") {
		return "", fmt.Errorf("invalid upload key prefix")
	}

	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	return cleanPrefix + "/" + id.String() + ext, nil
}
