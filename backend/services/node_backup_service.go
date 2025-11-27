package services

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gorm"
	appConfig "smartdns-manager/config"
	"smartdns-manager/models"
)

// NodeBackupService 节点备份服务
type NodeBackupService struct {
	db            *gorm.DB
	config        *appConfig.Config
	s3            *S3Service
	backupService *BackupService
}

// NewNodeBackupService 创建节点备份服务
func NewNodeBackupService(db *gorm.DB, config *appConfig.Config, s3 *S3Service) (*NodeBackupService, error) {
	return &NodeBackupService{
		db:            db,
		config:        config,
		s3:            s3,
		backupService: NewBackupService(),
	}, nil
}

// BackupNodes 备份节点配置
func (s *NodeBackupService) BackupNodes(ctx context.Context, config models.NodeBackupConfig) (string, error) {
	// 获取要备份的节点
	var nodes []models.Node
	query := s.db
	
	if len(config.NodeIDs) > 0 {
		query = query.Where("id IN ?", config.NodeIDs)
	}
	
	if err := query.Find(&nodes).Error; err != nil {
		return "", fmt.Errorf("查询节点失败: %w", err)
	}
	
	if len(nodes) == 0 {
		return "没有找到要备份的节点", nil
	}
	
	backedUpCount := 0
	var errors []string
	
	for _, node := range nodes {
		if err := s.backupSingleNode(ctx, node, config); err != nil {
			log.Printf("❌ 备份节点失败 [%s]: %v", node.Name, err)
			errors = append(errors, fmt.Sprintf("%s: %v", node.Name, err))
		} else {
			backedUpCount++
			log.Printf("✅ 节点备份成功: %s", node.Name)
		}
	}
	
	result := fmt.Sprintf("节点备份完成: 成功 %d/%d", backedUpCount, len(nodes))
	if len(errors) > 0 {
		result += fmt.Sprintf(", 失败: %v", errors)
	}
	
	return result, nil
}

// backupSingleNode 备份单个节点
func (s *NodeBackupService) backupSingleNode(ctx context.Context, node models.Node, config models.NodeBackupConfig) error {
	// 根据配置决定存储类型
	var storage BackupStorage
	var storageType string
	var err error
	
	if config.StorageType == "s3" && (config.S3Config != models.S3Config{}) {
		// 使用配置中的S3设置创建临时S3存储
		tempS3Config := &appConfig.StorageConfig{
			Type:        "s3",
			S3Region:    config.S3Config.Region,
			S3Bucket:    config.S3Config.Bucket,
			S3AccessKey: config.S3Config.AccessKey,
			S3SecretKey: config.S3Config.SecretKey,
			S3Endpoint:  config.S3Config.Endpoint,
		}
		
		storage, err = NewS3BackupStorage(tempS3Config)
		if err != nil {
			return fmt.Errorf("初始化S3存储失败: %w", err)
		}
		storageType = "s3"
	} else {
		// 使用本地存储
		storage, err = NewLocalBackupStorageWithNode(&node)
		if err != nil {
			return fmt.Errorf("初始化本地存储失败: %w", err)
		}
		storageType = "local"
	}
	defer storage.Close()
	
	// 使用通用备份服务执行备份
	backup, err := s.backupService.PerformNodeBackup(ctx, &node, storage, storageType, "自动任务备份", "", true)
	if err != nil {
		return fmt.Errorf("执行备份失败: %w", err)
	}

	// 保存备份记录到数据库
	if err := s.db.Create(backup).Error; err != nil {
		// 尝试清理已上传的文件
		storage.Delete(ctx, backup.Path)
		return fmt.Errorf("保存备份记录失败: %w", err)
	}
	
	log.Printf("✅ 节点备份成功: %s -> %s (存储类型: %s)", node.Name, backup.Path, storageType)
	return nil
}