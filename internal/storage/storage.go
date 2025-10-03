package storage

import (
        "context"
        "fmt"
        "strings"
        "time"

        "github.com/minio/minio-go/v7"
        "github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
        client     *minio.Client
        bucketName string
}

func NewStorage(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*Storage, error) {
        client, err := minio.New(endpoint, &minio.Options{
                Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
                Secure: useSSL,
        })
        if err != nil {
                return nil, fmt.Errorf("failed to create minio client: %w", err)
        }

        ctx := context.Background()
        exists, err := client.BucketExists(ctx, bucketName)
        if err != nil {
                return nil, fmt.Errorf("failed to check bucket: %w", err)
        }

        if !exists {
                err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
                if err != nil {
                        return nil, fmt.Errorf("failed to create bucket: %w", err)
                }
        }

        return &Storage{
                client:     client,
                bucketName: bucketName,
        }, nil
}

func (s *Storage) GeneratePresignedUploadURL(ctx context.Context, objectKey, contentType string, maxSize int64) (string, error) {
        allowedTypes := []string{"image/", "video/"}
        allowed := false
        for _, prefix := range allowedTypes {
                if strings.HasPrefix(contentType, prefix) {
                        allowed = true
                        break
                }
        }
        if !allowed {
                return "", fmt.Errorf("content type %s not allowed", contentType)
        }

        presignedURL, err := s.client.PresignedPutObject(ctx, s.bucketName, objectKey, 15*time.Minute)
        if err != nil {
                return "", fmt.Errorf("failed to generate presigned URL: %w", err)
        }

        return presignedURL.String(), nil
}

func (s *Storage) GetObjectURL(objectKey string) string {
        return fmt.Sprintf("https://%s/%s/%s", s.client.EndpointURL().Host, s.bucketName, objectKey)
}
