package s3spaces

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/TechXTT/bazaar-backend/pkg/app"
	"github.com/TechXTT/bazaar-backend/services/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

type (
	S3Spaces interface {
		// SaveImage validates that the uploaded file is an allowed image type,
		// generates a server-side object key (keyPrefix/<uuid>.<ext>) that never
		// trusts the client filename, and persists it via the configured driver.
		// keyPrefix is sanitized to reject path traversal. This is the only
		// supported entrypoint for user uploads (BE-5).
		SaveImage(file multipart.File, keyPrefix string) (string, error)
	}

	s3spaces struct {
		cfg    config.Config
		client *s3.S3
	}
)

func init() {
	// Provide dependencies during app boot process
	app.HookBoot.Listen(func(e hooks.Event[*do.Injector]) {
		do.Provide(e.Msg, NewS3Spaces)
	})
}

func NewS3Spaces(i *do.Injector) (S3Spaces, error) {
	cfg := do.MustInvoke[config.Config](i)
	s3SpacesCfg := cfg.GetS3Spaces()

	switch s3SpacesCfg.StorageDriver {
	case "local":
		if err := os.MkdirAll(s3SpacesCfg.LocalUploadDir, 0o755); err != nil {
			return nil, err
		}
		return &s3spaces{cfg: cfg}, nil
	case "ipfs":
		return &s3spaces{cfg: cfg}, nil
	}

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(s3SpacesCfg.SpacesKey, s3SpacesCfg.SpacesSecret, ""),
		Endpoint:         aws.String(s3SpacesCfg.SpacesEndpoint),
		Region:           aws.String(s3SpacesCfg.SpacesRegion),
		S3ForcePathStyle: aws.Bool(false),
	}

	newSession, err := session.NewSession(s3Config)
	if err != nil {
		return nil, err
	}

	return &s3spaces{
		cfg:    cfg,
		client: s3.New(newSession),
	}, nil
}

func (s *s3spaces) SaveImage(file multipart.File, keyPrefix string) (string, error) {
	// BE-5: validate content type against an image allowlist for ALL drivers and
	// derive the extension from the sniffed bytes, never the client filename.
	ext, err := detectImageExt(file)
	if err != nil {
		return "", err
	}

	key, err := buildObjectKey(keyPrefix, ext)
	if err != nil {
		return "", err
	}

	switch s.cfg.GetS3Spaces().StorageDriver {
	case "local":
		return s.saveLocalFile(file, key)
	case "ipfs":
		return s.saveIPFS(file, key)
	default:
		return s.saveS3(file, key)
	}
}

func (s *s3spaces) saveS3(file multipart.File, key string) (string, error) {
	s3SpacesCfg := s.cfg.GetS3Spaces()

	// BE-5: defence-in-depth — reject traversal even though SaveImage already
	// produces a server-controlled key. An object key must not contain "..", be
	// absolute, or be empty after cleaning.
	cleanKey := strings.TrimPrefix(path.Clean("/"+key), "/")
	if cleanKey == "" || cleanKey == "." || strings.Contains(cleanKey, "..") {
		return "", fmt.Errorf("invalid object key")
	}

	_, err := s.client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s3SpacesCfg.SpacesName),
		Key:    aws.String(cleanKey),
		Body:   file,
		ACL:    aws.String("public-read"),
	})

	if err != nil {
		return "", err
	}

	if s3SpacesCfg.SpacesCDNBase == "" {
		return cleanKey, nil
	}

	return fmt.Sprintf("%s/%s", strings.TrimRight(s3SpacesCfg.SpacesCDNBase, "/"), cleanKey), nil
}

func (s *s3spaces) saveLocalFile(file multipart.File, path string) (string, error) {
	cfg := s.cfg.GetS3Spaces()
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("invalid upload path")
	}

	destination := filepath.Join(cfg.LocalUploadDir, cleanPath)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", err
	}

	out, err := os.Create(destination)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/api/uploads/%s", strings.TrimRight(s.cfg.GetApp().BackendURL, "/"), filepath.ToSlash(cleanPath)), nil
}

func (s *s3spaces) saveIPFS(file multipart.File, _ string) (string, error) {
	cfg := s.cfg.GetS3Spaces()
	if cfg.PinataJWT == "" {
		return "", fmt.Errorf("PINATA_JWT not configured")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "evidence")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequest(http.MethodPost, "https://api.pinata.cloud/pinning/pinFileToIPFS", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.PinataJWT)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var pinataResp struct {
		IpfsHash string `json:"IpfsHash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pinataResp); err != nil {
		return "", err
	}
	if pinataResp.IpfsHash == "" {
		return "", fmt.Errorf("pinata returned empty CID")
	}

	return "ipfs://" + pinataResp.IpfsHash, nil
}
