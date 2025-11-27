package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smartdns-manager/models"
	"smartdns-manager/services"
)

type DatabaseBackupHandler struct {
	db            *gorm.DB
	backupService *services.DatabaseBackupService
}

func NewDatabaseBackupHandler(db *gorm.DB, backupService *services.DatabaseBackupService) *DatabaseBackupHandler {
	return &DatabaseBackupHandler{
		db:            db,
		backupService: backupService,
	}
}

// CreateBackupConfig 创建备份配置
// @Summary 创建备份配置
// @Tags DatabaseBackup
// @Accept json
// @Produce json
// @Param config body models.BackupConfigRequest true "备份配置"
// @Success 200 {object} models.BackupConfig
// @Router /api/database-backup/configs [post]
func (h *DatabaseBackupHandler) CreateBackupConfig(c *gin.Context) {
	var request models.BackupConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// 验证S3配置
	if request.S3Enabled {
		if request.S3AccessKey == "" || request.S3SecretKey == "" || request.S3Region == "" || request.S3Bucket == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "S3配置不完整"})
			return
		}
	}

	// 验证本地路径或S3至少启用一个
	if !request.S3Enabled && request.LocalPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "必须启用S3存储或配置本地存储路径"})
		return
	}

	// 转换通知渠道
	var notificationChannels string
	if len(request.NotificationChannels) > 0 {
		channelBytes, _ := json.Marshal(request.NotificationChannels)
		notificationChannels = string(channelBytes)
	}

	config := &models.BackupConfig{
		Name:                 request.Name,
		Enabled:              request.Enabled,
		BackupType:           request.BackupType,
		Schedule:             request.Schedule,
		RetentionDays:        request.RetentionDays,
		S3Enabled:            request.S3Enabled,
		S3AccessKey:          request.S3AccessKey,
		S3SecretKey:          request.S3SecretKey,
		S3Region:             request.S3Region,
		S3Bucket:             request.S3Bucket,
		S3Endpoint:           request.S3Endpoint,
		S3Prefix:             request.S3Prefix,
		LocalPath:            request.LocalPath,
		CompressionEnabled:   request.CompressionEnabled,
		CompressionLevel:     request.CompressionLevel,
		EncryptionEnabled:    request.EncryptionEnabled,
		EncryptionKey:        request.EncryptionKey,
		NotifyOnSuccess:      request.NotifyOnSuccess,
		NotifyOnFailure:      request.NotifyOnFailure,
		NotificationChannels: notificationChannels,
	}

	if err := h.db.Create(config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建备份配置失败: " + err.Error()})
		return
	}

	// 如果启用了配置，调度备份任务
	if config.Enabled {
		if err := h.backupService.UpdateBackupConfig(config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "调度备份任务失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "备份配置创建成功",
		"data":    config,
		"success": true,
	})
}

// UpdateBackupConfig 更新备份配置
// @Summary 更新备份配置
// @Tags DatabaseBackup
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param config body models.BackupConfigRequest true "备份配置"
// @Success 200 {object} models.BackupConfig
// @Router /api/database-backup/configs/{id} [put]
func (h *DatabaseBackupHandler) UpdateBackupConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	var config models.BackupConfig
	if err := h.db.First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份配置不存在"})
		return
	}

	var request models.BackupConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// 验证S3配置
	if request.S3Enabled {
		if request.S3AccessKey == "" || request.S3SecretKey == "" || request.S3Region == "" || request.S3Bucket == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "S3配置不完整"})
			return
		}
	}

	// 转换通知渠道
	var notificationChannels string
	if len(request.NotificationChannels) > 0 {
		channelBytes, _ := json.Marshal(request.NotificationChannels)
		notificationChannels = string(channelBytes)
	}

	// 更新配置
	config.Name = request.Name
	config.Enabled = request.Enabled
	config.BackupType = request.BackupType
	config.Schedule = request.Schedule
	config.RetentionDays = request.RetentionDays
	config.S3Enabled = request.S3Enabled
	config.S3AccessKey = request.S3AccessKey
	config.S3SecretKey = request.S3SecretKey
	config.S3Region = request.S3Region
	config.S3Bucket = request.S3Bucket
	config.S3Endpoint = request.S3Endpoint
	config.S3Prefix = request.S3Prefix
	config.LocalPath = request.LocalPath
	config.CompressionEnabled = request.CompressionEnabled
	config.CompressionLevel = request.CompressionLevel
	config.EncryptionEnabled = request.EncryptionEnabled
	config.EncryptionKey = request.EncryptionKey
	config.NotifyOnSuccess = request.NotifyOnSuccess
	config.NotifyOnFailure = request.NotifyOnFailure
	config.NotificationChannels = notificationChannels

	// 更新备份服务配置
	if err := h.backupService.UpdateBackupConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新备份配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "备份配置更新成功",
		"data":    config,
		"success": true,
	})
}

// GetBackupConfigs 获取备份配置列表
// @Summary 获取备份配置列表
// @Tags DatabaseBackup
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200
// @Router /api/database-backup/configs [get]
func (h *DatabaseBackupHandler) GetBackupConfigs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	var configs []models.BackupConfig
	var total int64

	offset := (page - 1) * pageSize

	if err := h.db.Model(&models.BackupConfig{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	if err := h.db.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "查询成功",
		"data": gin.H{
			"configs":   configs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
		"success": true,
	})
}

// GetBackupConfig 获取单个备份配置
// @Summary 获取单个备份配置
// @Tags DatabaseBackup
// @Produce json
// @Param id path int true "配置ID"
// @Success 200 {object} models.BackupConfig
// @Router /api/database-backup/configs/{id} [get]
func (h *DatabaseBackupHandler) GetBackupConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	var config models.BackupConfig
	if err := h.db.First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份配置不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "查询成功",
		"data":    config,
		"success": true,
	})
}

// DeleteBackupConfig 删除备份配置
// @Summary 删除备份配置
// @Tags DatabaseBackup
// @Produce json
// @Param id path int true "配置ID"
// @Success 200
// @Router /api/database-backup/configs/{id} [delete]
func (h *DatabaseBackupHandler) DeleteBackupConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	var config models.BackupConfig
	if err := h.db.First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份配置不存在"})
		return
	}

	// 删除配置
	if err := h.db.Delete(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "备份配置删除成功",
		"success": true})
}

// ManualBackup 手动触发备份
// @Summary 手动触发备份
// @Tags DatabaseBackup
// @Produce json
// @Param id path int true "配置ID"
// @Success 200
// @Router /api/database-backup/configs/{id}/backup [post]
func (h *DatabaseBackupHandler) ManualBackup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	if err := h.backupService.ManualBackup(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "触发备份失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true, "message": "备份任务已启动"})
}

// GetBackupHistory 获取备份历史
// @Summary 获取备份历史
// @Tags DatabaseBackup
// @Produce json
// @Param config_id query int false "配置ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200
// @Router /api/database-backup/history [get]
func (h *DatabaseBackupHandler) GetBackupHistory(c *gin.Context) {
	configID := c.Query("config_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	var history []models.BackupHistory
	var total int64

	offset := (page - 1) * pageSize
	query := h.db.Model(&models.BackupHistory{}).Preload("Config")

	if configID != "" {
		query = query.Where("config_id = ?", configID)
	}

	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&history).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "查询成功",
		"data": gin.H{
			"history":   history,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
		"success": true,
	})
}

// RestoreBackup 恢复备份
// @Summary 恢复备份
// @Tags DatabaseBackup
// @Accept json
// @Produce json
// @Param request body models.BackupRestoreRequest true "恢复请求"
// @Success 200
// @Router /api/database-backup/restore [post]
func (h *DatabaseBackupHandler) RestoreBackup(c *gin.Context) {
	var request models.BackupRestoreRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := h.backupService.RestoreBackup(&request); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复备份失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true, "message": "备份恢复成功"})
}

// GetBackupStats 获取备份统计信息
// @Summary 获取备份统计信息
// @Tags DatabaseBackup
// @Produce json
// @Success 200 {object} models.BackupStats
// @Router /api/database-backup/stats [get]
func (h *DatabaseBackupHandler) GetBackupStats(c *gin.Context) {
	stats, err := h.backupService.GetBackupStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计信息失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "查询成功",
		"data":    stats,
		"success": true,
	})
}

// TestS3Connection 测试S3连接
// @Summary 测试S3连接
// @Tags DatabaseBackup
// @Accept json
// @Produce json
// @Param config body models.BackupConfigRequest true "S3配置"
// @Success 200
// @Router /api/database-backup/test-s3 [post]
func (h *DatabaseBackupHandler) TestS3Connection(c *gin.Context) {
	var request models.BackupConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if !request.S3Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "S3未启用"})
		return
	}

	// 创建临时S3客户端测试连接
	s3Config := services.S3Config{
		AccessKey: request.S3AccessKey,
		SecretKey: request.S3SecretKey,
		Region:    request.S3Region,
		Bucket:    request.S3Bucket,
		Endpoint:  request.S3Endpoint,
	}

	_, err := services.NewS3Service(s3Config, h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建S3客户端失败: " + err.Error()})
		return
	}

	// 简单的连通性测试 - 如果能创建客户端就表示配置正确
	c.JSON(http.StatusOK, gin.H{
		"success": true, "message": "S3连接测试成功"})
}
