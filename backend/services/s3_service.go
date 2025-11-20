package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"smartdns-manager/models"
)

type S3Service struct {
	client *s3.Client
	bucket string
	region string
	db     *gorm.DB
}

type S3Config struct {
	AccessKey string
	SecretKey string
	Region    string
	Bucket    string
	Endpoint  string // 可选，用于兼容其他S3服务
}

func NewS3Service(cfg S3Config, db *gorm.DB) (*S3Service, error) {
	ctx := context.TODO()

	var awsCfg aws.Config
	var err error

	if cfg.Endpoint != "" {
		// 自定义endpoint（如MinIO）
		customResolver := aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           cfg.Endpoint,
					SigningRegion: cfg.Region,
				}, nil
			})

		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithEndpointResolverWithOptions(customResolver),
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					cfg.AccessKey,
					cfg.SecretKey,
					"",
				),
			),
		)
	} else {
		// 标准AWS S3
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					cfg.AccessKey,
					cfg.SecretKey,
					"",
				),
			),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	return &S3Service{
		client: client,
		bucket: cfg.Bucket,
		region: cfg.Region,
		db:     db,
	}, nil
}

// UploadFile 上传文件
func (s *S3Service) UploadFile(ctx context.Context, file multipart.File, fileHeader *multipart.FileHeader, folder string, userID uint) (*models.File, error) {
	// 生成唯一的文件key
	ext := filepath.Ext(fileHeader.Filename)
	s3Key := fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)

	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 上传到S3
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s3Key),
		Body:          bytes.NewReader(fileBytes),
		ContentType:   aws.String(fileHeader.Header.Get("Content-Type")),
		ContentLength: aws.Int64(fileHeader.Size),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	// 生成URL
	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, s3Key)

	// 保存到数据库
	fileModel := &models.File{
		FileName:  fileHeader.Filename,
		FileSize:  fileHeader.Size,
		FileType:  fileHeader.Header.Get("Content-Type"),
		S3Key:     s3Key,
		S3Bucket:  s.bucket,
		URL:       url,
		UserID:    userID,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.Create(fileModel).Error; err != nil {
		// 如果数据库保存失败，尝试删除S3文件
		s.DeleteFileFromS3(ctx, s3Key)
		return nil, fmt.Errorf("failed to save file record: %w", err)
	}

	return fileModel, nil
}

// GetPresignedURL 获取预签名URL（用于临时访问私有文件）
func (s *S3Service) GetPresignedURL(ctx context.Context, s3Key string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s3Key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}

// DownloadFile 下载文件
func (s *S3Service) DownloadFile(ctx context.Context, s3Key string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s3Key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return data, nil
}

// DeleteFile 删除文件（软删除）
func (s *S3Service) DeleteFile(ctx context.Context, fileID uint) error {
	var file models.File
	if err := s.db.First(&file, fileID).Error; err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// 软删除
	file.Status = "deleted"
	if err := s.db.Save(&file).Error; err != nil {
		return fmt.Errorf("failed to update file status: %w", err)
	}

	return nil
}

// DeleteFilePermanently 永久删除文件
func (s *S3Service) DeleteFilePermanently(ctx context.Context, fileID uint) error {
	var file models.File
	if err := s.db.First(&file, fileID).Error; err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// 从S3删除
	if err := s.DeleteFileFromS3(ctx, file.S3Key); err != nil {
		return err
	}

	// 从数据库删除
	if err := s.db.Delete(&file).Error; err != nil {
		return fmt.Errorf("failed to delete file record: %w", err)
	}

	return nil
}

// DeleteFileFromS3 从S3删除文件
func (s *S3Service) DeleteFileFromS3(ctx context.Context, s3Key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s3Key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// ListFiles 列出文件
func (s *S3Service) ListFiles(ctx context.Context, userID uint, page, pageSize int) ([]models.File, int64, error) {
	var files []models.File
	var total int64

	offset := (page - 1) * pageSize

	query := s.db.Model(&models.File{}).Where("user_id = ? AND status = ?", userID, "active")

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&files).Error; err != nil {
		return nil, 0, err
	}

	return files, total, nil
}

// GetFileByID 根据ID获取文件信息
func (s *S3Service) GetFileByID(ctx context.Context, fileID uint) (*models.File, error) {
	var file models.File
	if err := s.db.First(&file, fileID).Error; err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	return &file, nil
}
