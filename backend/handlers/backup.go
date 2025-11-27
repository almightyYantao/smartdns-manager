// handlers/backup_handler.go
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

type BackupHandler struct {
	db            *gorm.DB
	storageManager *services.BackupStorageManager
	backupService *services.BackupService
}

var (
	defaultBackupHandler *BackupHandler
	backupHandlerOnce    sync.Once
)

func initBackupHandler() {
	backupHandlerOnce.Do(func() {
		defaultBackupHandler = NewBackupHandler(database.DB)
	})
}

// 包级函数
func GetNodeBackups(c *gin.Context) {
	initBackupHandler()
	defaultBackupHandler.GetNodeBackups(c)
}

func CreateNodeBackup(c *gin.Context) {
	initBackupHandler()
	defaultBackupHandler.CreateNodeBackup(c)
}

func PreviewBackup(c *gin.Context) {
	initBackupHandler()
	defaultBackupHandler.PreviewBackup(c)
}

func RestoreNodeBackup(c *gin.Context) {
	initBackupHandler()
	defaultBackupHandler.RestoreNodeBackup(c)
}

func DeleteNodeBackup(c *gin.Context) {
	initBackupHandler()
	defaultBackupHandler.DeleteNodeBackup(c)
}

func DownloadBackup(c *gin.Context) {
	initBackupHandler()
	defaultBackupHandler.DownloadBackup(c)
}

func NewBackupHandler(db *gorm.DB) *BackupHandler {
	return &BackupHandler{
		db:             db,
		storageManager: services.NewBackupStorageManager(),
		backupService:  services.NewBackupService(),
	}
}

// ═══════════════════════════════════════════════════════════════
// 获取节点备份列表
// ═══════════════════════════════════════════════════════════════

// GetNodeBackups 获取节点的备份列表
// GET /api/nodes/:id/backups
func (h *BackupHandler) GetNodeBackups(c *gin.Context) {
	nodeID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	// 验证节点是否存在
	var node models.Node
	if err := h.db.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	// 查询备份列表
	query := h.db.Model(&models.Backup{}).
		Where("node_id = ? AND is_deleted = ?", nodeID, false)

	// 获取总数
	var total int64
	query.Count(&total)

	// 分页查询
	var backups []models.Backup
	offset := (page - 1) * pageSize
	if err := query.Preload("Node").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&backups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 刷新 S3 下载链接
	h.refreshS3DownloadURLs(backups)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"list":      backups,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
		"success": true,
	})
}

// ═══════════════════════════════════════════════════════════════
// 创建节点备份
// ═══════════════════════════════════════════════════════════════

type CreateBackupRequest struct {
	Comment string `json:"comment"`
	Tags    string `json:"tags"`
}

// CreateNodeBackup 创建节点备份
// POST /api/nodes/:id/backups
func (h *BackupHandler) CreateNodeBackup(c *gin.Context) {
	nodeID := c.Param("id")

	var req CreateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查询节点
	var node models.Node
	if err := h.db.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	// 获取存储实例
	storage, storageType, err := h.storageManager.GetStorage(&node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("初始化存储失败: %v", err)})
		return
	}
	defer storage.Close()

	// 执行备份
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	backup, err := h.performBackup(ctx, &node, storage, storageType, req.Comment, req.Tags, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "备份创建成功",
		"data":    backup,
		"success": true,
	})
}

// performBackup 执行备份操作
func (h *BackupHandler) performBackup(
	ctx context.Context,
	node *models.Node,
	storage services.BackupStorage,
	storageType string,
	comment string,
	tags string,
	isAuto bool,
) (*models.Backup, error) {
	// 使用通用备份服务
	backup, err := h.backupService.PerformNodeBackup(ctx, node, storage, storageType, comment, tags, isAuto)
	if err != nil {
		return nil, err
	}

	// S3 存储的额外信息（Handler 特有逻辑）
	if storageType == "s3" {
		cfg := h.storageManager.GetConfig()
		backup.S3Bucket = cfg["s3_bucket"].(string)
		backup.S3Region = cfg["s3_region"].(string)
	}

	// 保存到数据库
	if err := h.db.Create(backup).Error; err != nil {
		// 尝试清理已上传的文件
		storage.Delete(ctx, backup.Path)
		return nil, fmt.Errorf("保存备份记录失败: %w", err)
	}

	return backup, nil
}

// ═══════════════════════════════════════════════════════════════
// 预览备份
// ═══════════════════════════════════════════════════════════════

// type BackupPath struct {
// 	BackupID uint `json:"backup_id" binding:"required"`
// }

type PreviewBackupRequest struct {
	BackupID uint `json:"backup_id" binding:"required"`
}

// PreviewBackup 预览备份内容
// POST /api/nodes/:id/backups/preview
func (h *BackupHandler) PreviewBackup(c *gin.Context) {
	nodeID := c.Param("id")

	var req PreviewBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查询备份
	var backup models.Backup
	backupID := req.BackupID
	if err := h.db.Preload("Node").First(&backup, backupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份不存在"})
		return
	}

	// 验证备份属于该节点
	if strconv.Itoa(int(backup.NodeID)) != nodeID {
		c.JSON(http.StatusForbidden, gin.H{"error": "备份不属于该节点"})
		return
	}

	// 获取存储实例
	storage, err := h.storageManager.GetStorageForBackup(&backup, &backup.Node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("初始化存储失败: %v", err)})
		return
	}
	defer storage.Close()

	// 读取备份内容
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	content, err := storage.Load(ctx, backup.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取备份失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"backup_id":  backup.ID,
			"name":       backup.Name,
			"content":    string(content),
			"size":       backup.Size,
			"created_at": backup.CreatedAt,
		},
		"success": true,
	})
}

// ═══════════════════════════════════════════════════════════════
// 恢复备份
// ═══════════════════════════════════════════════════════════════

type RestoreBackupRequest struct {
	BackupID uint `json:"backup_id" binding:"required"`
}

// RestoreNodeBackup 恢复节点备份
// POST /api/nodes/:id/backups/restore
func (h *BackupHandler) RestoreNodeBackup(c *gin.Context) {
	nodeID := c.Param("id")

	var req RestoreBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查询备份
	var backup models.Backup
	if err := h.db.Preload("Node").First(&backup, req.BackupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份不存在"})
		return
	}

	// 验证备份属于该节点
	if strconv.Itoa(int(backup.NodeID)) != nodeID {
		c.JSON(http.StatusForbidden, gin.H{"error": "备份不属于该节点"})
		return
	}

	// 获取存储实例
	storage, err := h.storageManager.GetStorageForBackup(&backup, &backup.Node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("初始化存储失败: %v", err)})
		return
	}
	defer storage.Close()

	// 读取备份内容
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	content, err := storage.Load(ctx, backup.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取备份失败: %v", err)})
		return
	}

	// 创建 SSH 客户端
	sshClient, err := services.NewSSHClient(&backup.Node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建SSH连接失败: %v", err)})
		return
	}
	defer sshClient.Close()

	// 创建临时文件并写入内容
	tmpFile := fmt.Sprintf("/tmp/smartdns_restore_%d.conf", time.Now().Unix())
	if err := sshClient.WriteFile(tmpFile, string(content)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("写入临时文件失败: %v", err)})
		return
	}

	// 备份当前配置
	backupCmd := fmt.Sprintf("sudo cp /etc/smartdns/smartdns.conf /etc/smartdns/smartdns.conf.before-restore-%d", time.Now().Unix())
	if _, err := sshClient.ExecuteCommand(backupCmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("备份当前配置失败: %v", err)})
		return
	}

	// 恢复配置
	restoreCmd := fmt.Sprintf("sudo mv %s /etc/smartdns/smartdns.conf && sudo chmod 644 /etc/smartdns/smartdns.conf", tmpFile)
	if _, err := sshClient.ExecuteCommand(restoreCmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("恢复配置失败: %v", err)})
		return
	}

	// 重启 SmartDNS 服务
	if _, err := sshClient.ExecuteCommand("sudo systemctl restart smartdns"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("重启服务失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "备份恢复成功",
		"data": gin.H{
			"backup_id": backup.ID,
			"node_id":   backup.NodeID,
		},
		"success": true,
	})
}

// ═══════════════════════════════════════════════════════════════
// 删除备份
// ═══════════════════════════════════════════════════════════════

type DeleteBackupRequest struct {
	BackupIDs []uint `json:"backup_ids" binding:"required"` // 支持批量删除
}

// DeleteNodeBackup 删除节点备份
// DELETE /api/nodes/:id/backups
func (h *BackupHandler) DeleteNodeBackup(c *gin.Context) {
	nodeID := c.Param("id")

	var req DeleteBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查询备份
	var backups []models.Backup
	if err := h.db.Preload("Node").
		Where("id IN ? AND node_id = ? AND is_deleted = ?", req.BackupIDs, nodeID, false).
		Find(&backups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(backups) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到可删除的备份"})
		return
	}

	successCount := 0
	failedCount := 0

	for _, backup := range backups {
		// 获取存储实例
		storage, err := h.storageManager.GetStorageForBackup(&backup, &backup.Node)
		if err != nil {
			failedCount++
			continue
		}

		// 删除存储中的文件
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		if err := storage.Delete(ctx, backup.Path); err != nil {
			// 记录错误但继续
			fmt.Printf("删除存储文件失败: %v\n", err)
		}
		cancel()
		storage.Close()

		// 软删除数据库记录
		backup.IsDeleted = true
		if err := h.db.Save(&backup).Error; err != nil {
			failedCount++
		} else {
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("删除完成：成功 %d，失败 %d", successCount, failedCount),
		"data": gin.H{
			"success": successCount,
			"failed":  failedCount,
		},
		"success": true,
	})
}

// ═══════════════════════════════════════════════════════════════
// 下载备份
// ═══════════════════════════════════════════════════════════════

// DownloadBackup 下载备份文件
// GET /api/nodes/:id/backups/download?backup_id=xxx
func (h *BackupHandler) DownloadBackup(c *gin.Context) {
	nodeID := c.Param("id")
	backupID := c.Query("backup_id")

	if backupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 backup_id 参数"})
		return
	}

	// 查询备份
	var backup models.Backup
	if err := h.db.Preload("Node").First(&backup, backupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份不存在"})
		return
	}

	// 验证备份属于该节点
	if strconv.Itoa(int(backup.NodeID)) != nodeID {
		c.JSON(http.StatusForbidden, gin.H{"error": "备份不属于该节点"})
		return
	}

	// 如果是 S3 且有有效的下载链接，重定向
	if backup.StorageType == "s3" && backup.DownloadURL != "" {
		c.Redirect(http.StatusFound, backup.DownloadURL)
		return
	}

	// 获取存储实例
	storage, err := h.storageManager.GetStorageForBackup(&backup, &backup.Node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("初始化存储失败: %v", err)})
		return
	}
	defer storage.Close()

	// 读取备份内容
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	content, err := storage.Load(ctx, backup.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取备份失败: %v", err)})
		return
	}

	// 设置响应头
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", backup.Name))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", len(content)))

	c.Data(http.StatusOK, "application/octet-stream", content)
}

// ═══════════════════════════════════════════════════════════════
// 辅助方法
// ═══════════════════════════════════════════════════════════════

// refreshS3DownloadURLs 批量刷新 S3 下载链接
func (h *BackupHandler) refreshS3DownloadURLs(backups []models.Backup) {
	if h.storageManager.GetStorageType() != "s3" {
		return
	}

	storage, err := services.NewS3BackupStorage(h.storageManager.GetStorageConfig())
	if err != nil {
		return
	}
	defer storage.Close()

	ctx := context.Background()
	for i := range backups {
		if backups[i].StorageType == "s3" {
			url, err := storage.GetDownloadURL(ctx, backups[i].S3Key, 24*time.Hour)
			if err == nil {
				backups[i].DownloadURL = url
			}
		}
	}
}

// refreshS3DownloadURL 刷新单个 S3 下载链接
func (h *BackupHandler) refreshS3DownloadURL(backup *models.Backup) {
	if backup.StorageType != "s3" {
		return
	}

	storage, err := services.NewS3BackupStorage(h.storageManager.GetStorageConfig())
	if err != nil {
		return
	}
	defer storage.Close()

	ctx := context.Background()
	url, err := storage.GetDownloadURL(ctx, backup.S3Key, 24*time.Hour)
	if err == nil {
		backup.DownloadURL = url
	}
}

// ═══════════════════════════════════════════════════════════════
// 获取存储信息
// ═══════════════════════════════════════════════════════════════

// GetStorageInfo 获取存储配置信息
// GET /api/backups/storage-info
func (h *BackupHandler) GetStorageInfo(c *gin.Context) {
	config := h.storageManager.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"data":    config,
		"success": true,
	})
}
