package handlers

import (
	"net/http"
	"strconv"

	"smartdns-manager/database"
	"smartdns-manager/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetGroups 获取所有分组
func GetGroups(c *gin.Context) {
	var groups []models.DNSGroup

	if err := database.DB.Order("is_system DESC, name ASC").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取分组列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    groups,
		"total":   len(groups),
	})
}

// AddGroup 添加分组
func AddGroup(c *gin.Context) {
	var group models.DNSGroup

	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 检查名称是否已存在
	var existing models.DNSGroup
	if err := database.DB.Where("name = ?", group.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "该分组名称已存在",
		})
		return
	}

	// 用户创建的分组不是系统分组
	group.IsSystem = false

	if err := database.DB.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "添加分组失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "分组添加成功",
		"data":    group,
	})
}

// UpdateGroup 更新分组
func UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	groupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的分组ID",
		})
		return
	}

	var group models.DNSGroup
	if err := database.DB.First(&group, groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "分组不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询分组失败",
		})
		return
	}

	// 系统分组不允许修改名称
	var updateData models.DNSGroup
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	if group.IsSystem && updateData.Name != group.Name {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "系统分组不允许修改名称",
		})
		return
	}

	// 更新字段
	if updateData.Name != "" {
		group.Name = updateData.Name
	}
	group.Description = updateData.Description
	group.Color = updateData.Color

	if err := database.DB.Save(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "更新分组失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "分组更新成功",
		"data":    group,
	})
}

// DeleteGroup 删除分组
func DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	groupID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的分组ID",
		})
		return
	}

	var group models.DNSGroup
	if err := database.DB.First(&group, groupID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "分组不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询分组失败",
		})
		return
	}

	// 系统分组不允许删除
	if group.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "系统分组不允许删除",
		})
		return
	}

	if err := database.DB.Delete(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除分组失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "分组删除成功",
	})
}
