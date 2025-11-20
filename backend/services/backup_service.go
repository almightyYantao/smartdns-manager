// services/backup_storage.go
package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	appConfig "smartdns-manager/config"
	"smartdns-manager/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BackupStorage 存储接口
type BackupStorage interface {
	Save(ctx context.Context, content []byte, filename string) (string, error)
	Load(ctx context.Context, path string) ([]byte, error)
	Delete(ctx context.Context, path string) error
	GetDownloadURL(ctx context.Context, path string, expiration time.Duration) (string, error)
	Close() error // 添加资源清理方法
}

// ═══════════════════════════════════════════════════════════════
// 本地存储实现
// ═══════════════════════════════════════════════════════════════

type LocalBackupStorage struct {
	sshClient  *SSHClient
	ownsClient bool // 标记是否拥有 client 的所有权
}

func NewLocalBackupStorage(client *SSHClient) *LocalBackupStorage {
	return &LocalBackupStorage{
		sshClient:  client,
		ownsClient: false,
	}
}

func NewLocalBackupStorageWithNode(node *models.Node) (*LocalBackupStorage, error) {
	client, err := NewSSHClient(node)
	if err != nil {
		return nil, fmt.Errorf("创建SSH客户端失败: %w", err)
	}
	return &LocalBackupStorage{
		sshClient:  client,
		ownsClient: true,
	}, nil
}

func (l *LocalBackupStorage) Save(ctx context.Context, content []byte, filename string) (string, error) {
	// 本地存储时，文件已经由 SSH 操作保存，这里只返回路径
	return filename, nil
}

func (l *LocalBackupStorage) Load(ctx context.Context, path string) ([]byte, error) {
	cmd := fmt.Sprintf("sudo cat %s", path)
	content, err := l.sshClient.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("读取备份文件失败: %w", err)
	}
	return []byte(content), nil
}

func (l *LocalBackupStorage) Delete(ctx context.Context, path string) error {
	// 删除备份文件和备注文件
	cmd := fmt.Sprintf("sudo rm -f %s %s.comment", path, path)
	_, err := l.sshClient.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("删除备份文件失败: %w", err)
	}
	return nil
}

// GetStorageConfig 获取原始存储配置
func (m *BackupStorageManager) GetStorageConfig() *appConfig.StorageConfig {
	return m.config
}

func (l *LocalBackupStorage) GetDownloadURL(ctx context.Context, path string, expiration time.Duration) (string, error) {
	// 本地存储不支持预签名 URL，需要通过 API 下载
	return "", nil
}

func (l *LocalBackupStorage) Close() error {
	if l.ownsClient && l.sshClient != nil {
		return l.sshClient.Close()
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════
// S3存储实现
// ═══════════════════════════════════════════════════════════════

type S3BackupStorage struct {
	client *s3.Client
	bucket string
	region string
	prefix string
	config *appConfig.StorageConfig
}

func NewS3BackupStorage(cfg *appConfig.StorageConfig) (*S3BackupStorage, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	ctx := context.TODO()
	var awsCfg aws.Config
	var err error

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.S3Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.S3AccessKey,
				cfg.S3SecretKey,
				"",
			),
		),
	}

	// 自定义endpoint（支持MinIO等）
	if cfg.S3Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               cfg.S3Endpoint,
					SigningRegion:     cfg.S3Region,
					HostnameImmutable: true, // 对MinIO很重要
				}, nil
			})
		opts = append(opts, config.WithEndpointResolverWithOptions(customResolver))
	}

	awsCfg, err = config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("加载AWS配置失败: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// MinIO 兼容性设置
		if cfg.S3Endpoint != "" {
			o.UsePathStyle = true
		}
	})

	return &S3BackupStorage{
		client: client,
		bucket: cfg.S3Bucket,
		region: cfg.S3Region,
		config: cfg,
	}, nil
}

func (s *S3BackupStorage) Save(ctx context.Context, content []byte, filename string) (string, error) {
	// 生成 S3 Key，按日期组织
	s3Key := fmt.Sprintf("smartdns-backups/%s/%s",
		time.Now().Format("2006-01-02"), filename)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s3Key),
		Body:          bytes.NewReader(content),
		ContentType:   aws.String("application/octet-stream"),
		ContentLength: aws.Int64(int64(len(content))),
		Metadata: map[string]string{
			"original-filename": filename,
			"upload-time":       time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		return "", fmt.Errorf("上传到S3失败 [bucket=%s, key=%s]: %w", s.bucket, s3Key, err)
	}

	return s3Key, nil
}

func (s *S3BackupStorage) Load(ctx context.Context, path string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("从S3下载失败 [bucket=%s, key=%s]: %w", s.bucket, path, err)
	}
	defer result.Body.Close()

	content, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("读取S3对象内容失败: %w", err)
	}

	return content, nil
}

func (s *S3BackupStorage) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("从S3删除失败 [bucket=%s, key=%s]: %w", s.bucket, path, err)
	}
	return nil
}

func (s *S3BackupStorage) GetDownloadURL(ctx context.Context, path string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})

	if err != nil {
		return "", fmt.Errorf("生成预签名URL失败: %w", err)
	}

	return request.URL, nil
}

func (s *S3BackupStorage) Close() error {
	// S3 client 不需要显式关闭
	return nil
}

// ═══════════════════════════════════════════════════════════════
// 存储管理器
// ═══════════════════════════════════════════════════════════════

type BackupStorageManager struct {
	config *appConfig.StorageConfig
}

func NewBackupStorageManager() *BackupStorageManager {
	return &BackupStorageManager{
		config: appConfig.LoadStorageConfig(),
	}
}

// GetStorage 获取存储实例
func (m *BackupStorageManager) GetStorage(node *models.Node) (BackupStorage, string, error) {
	println(m.config)
	if m.config.IsS3Enabled() {
		storage, err := NewS3BackupStorage(m.config)
		if err != nil {
			return nil, "", fmt.Errorf("初始化S3存储失败: %w", err)
		}
		return storage, "s3", nil
	}

	// 默认使用本地存储
	storage, err := NewLocalBackupStorageWithNode(node)
	if err != nil {
		return nil, "", fmt.Errorf("初始化本地存储失败: %w", err)
	}
	return storage, "local", nil
}

// GetStorageForBackup 根据备份记录获取存储实例
func (m *BackupStorageManager) GetStorageForBackup(backup *models.Backup, node *models.Node) (BackupStorage, error) {
	if backup.StorageType == "s3" {
		return NewS3BackupStorage(m.config)
	}
	return NewLocalBackupStorageWithNode(node)
}

// GetStorageType 获取当前存储类型
func (m *BackupStorageManager) GetStorageType() string {
	return m.config.Type
}

// GetConfig 获取存储配置（脱敏）
func (m *BackupStorageManager) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":          m.config.Type,
		"s3_region":     m.config.S3Region,
		"s3_bucket":     m.config.S3Bucket,
		"s3_endpoint":   m.config.S3Endpoint,
		"is_s3_enabled": m.config.IsS3Enabled(),
	}
}

// ValidateConfig 验证配置
func (m *BackupStorageManager) ValidateConfig() error {
	return m.config.Validate()
}
