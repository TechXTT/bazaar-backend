package s3spaces

import (
	"mime/multipart"

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
		// SaveFile saves a file to the object storage
		SaveFile(file multipart.File, filepath string) (string, error)
	}

	s3spaces struct {
		cfg config.Config
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

	return &s3spaces{
		cfg: cfg,
	}, nil
}

func (s *s3spaces) SaveFile(file multipart.File, filepath string) (string, error) {
	s3SpacesCfg := s.cfg.GetS3Spaces()

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(s3SpacesCfg.SpacesKey, s3SpacesCfg.SpacesSecret, ""),
		Endpoint:         aws.String("https://fra1.digitaloceanspaces.com"),
		Region:           aws.String("us-east-1"),
		S3ForcePathStyle: aws.Bool(false),
	}

	newSession, err := session.NewSession(s3Config)
	if err != nil {
		return "", err
	}
	s3Client := s3.New(newSession)

	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s3SpacesCfg.SpacesName),
		Key:    aws.String(filepath),
		Body:   file,
		ACL:    aws.String("public-read"),
	})

	if err != nil {
		return "", err
	}

	return filepath, nil
}
