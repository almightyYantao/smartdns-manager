package handlers

import (
	"log"
	"net/http"
	"smartdns-manager/services"
	"strconv"

	"github.com/gin-gonic/gin"
)

var versionService *services.VersionService

// InitVersionHandler 初始化版本服务
func InitVersionHandler(currentVersion string) {
	// 替换为你的 GitHub 仓库信息
	versionService = services.NewVersionService(currentVersion, "almightyyantao", "smartdns-manager")
	log.Printf("🔧 版本服务初始化完成，当前版本: %s", currentVersion)
}

// CheckVersion 检查版本更新
func CheckVersion(c *gin.Context) {
	if versionService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "版本服务未初始化",
		})
		return
	}

	notification, err := versionService.CheckForUpdates()
	if err != nil {
		log.Printf("❌ 检查版本更新失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "检查版本失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    notification,
	})
}

// GetSystemInfo 获取系统信息
func GetSystemInfo(c *gin.Context) {
	if versionService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "版本服务未初始化",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"current_version": versionService.GetCurrentVersion(),
			"repository":      "almightyyantao/smartdns-manager",
		},
	})
}

// GetVersionHistory 获取版本历史
func GetVersionHistory(c *gin.Context) {
	if versionService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "版本服务未初始化",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	versions, err := versionService.GetVersionHistory(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取版本历史失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"versions": versions,
			"total":    len(versions),
		},
	})
}
