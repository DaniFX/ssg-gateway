package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
	"github.com/ssg/ssg-gateway/internal/services"
)

type UserHandler struct {
	userRepo        repository.UserRepository
	roleRepo        repository.RoleRepository
	firebaseService *services.FirebaseService
	appID           string
}

func NewUserHandler(userRepo repository.UserRepository, roleRepo repository.RoleRepository, firebaseService *services.FirebaseService, appID string) *UserHandler {
	return &UserHandler{
		userRepo:        userRepo,
		roleRepo:        roleRepo,
		firebaseService: firebaseService,
		appID:           appID,
	}
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

type CreateUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role"`
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Email and password are required",
			},
		})
		return
	}

	userRecord, err := h.firebaseService.CreateUser(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "CREATE_FAILED",
				"message": "Failed to create user: " + err.Error(),
			},
		})
		return
	}

	role := req.Role
	if role == "" {
		role = "viewer"
	}

	user := &models.User{
		ID:    userRecord.UID,
		Email: req.Email,
		Apps: map[string]models.AppRole{
			h.appID: {Role: role, AddedAt: time.Now()},
		},
	}

	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "CREATE_FAILED",
				"message": "Failed to save user",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"role":  role,
		},
	})
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	users, err := h.userRepo.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch users",
			},
		})
		return
	}

	for i := range users {
		if users[i].Email == "" && users[i].ID != "" {
			email, err := h.firebaseService.GetUserEmail(context.Background(), users[i].ID)
			if err == nil && email != "" {
				users[i].Email = email
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    users,
	})
}

func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NOT_FOUND",
				"message": "User not found",
			},
		})
		return
	}

	if user.Email == "" && user.ID != "" {
		email, err := h.firebaseService.GetUserEmail(context.Background(), user.ID)
		if err == nil && email != "" {
			user.Email = email
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

func (h *UserHandler) UpdateUserRole(c *gin.Context) {
	userID := c.Param("id")

	var req UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Role is required",
			},
		})
		return
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), req.Role)
	if err != nil || role == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_ROLE",
				"message": "Role does not exist",
			},
		})
		return
	}

	err = h.userRepo.SetUserAppRole(c.Request.Context(), userID, h.appID, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to update user role",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"userId": userID,
			"appId":  h.appID,
			"role":   req.Role,
		},
	})
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")

	if err := h.firebaseService.DeleteUser(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to delete user from Firebase",
			},
		})
		return
	}

	err := h.userRepo.Delete(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to delete user from database",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"message": "User deleted"},
	})
}

type RoleHandler struct {
	roleRepo repository.RoleRepository
}

func NewRoleHandler(roleRepo repository.RoleRepository) *RoleHandler {
	return &RoleHandler{
		roleRepo: roleRepo,
	}
}

func (h *RoleHandler) ListRoles(c *gin.Context) {
	roles, err := h.roleRepo.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch roles",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    roles,
	})
}

func (h *RoleHandler) GetRole(c *gin.Context) {
	roleID := c.Param("id")

	role, err := h.roleRepo.GetByID(c.Request.Context(), roleID)
	if err != nil || role == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NOT_FOUND",
				"message": "Role not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    role,
	})
}

func (h *RoleHandler) CreateRole(c *gin.Context) {
	var role models.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request body",
			},
		})
		return
	}

	if role.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Role ID is required",
			},
		})
		return
	}

	err := h.roleRepo.Create(c.Request.Context(), &role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to create role",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    role,
	})
}

func (h *RoleHandler) UpdateRole(c *gin.Context) {
	roleID := c.Param("id")

	var role models.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request body",
			},
		})
		return
	}

	role.ID = roleID

	err := h.roleRepo.Update(c.Request.Context(), &role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to update role",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    role,
	})
}

func (h *RoleHandler) DeleteRole(c *gin.Context) {
	roleID := c.Param("id")

	err := h.roleRepo.Delete(c.Request.Context(), roleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to delete role",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"message": "Role deleted"},
	})
}
