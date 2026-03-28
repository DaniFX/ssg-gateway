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
		HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to bind create user request", "Email and password are required", err)
		return
	}

	userRecord, err := h.firebaseService.CreateUser(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		HandleError(c, http.StatusInternalServerError, "CREATE_FAILED", "Failed to create Firebase user", "Failed to create user", err)
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
		HandleError(c, http.StatusInternalServerError, "CREATE_FAILED", "Failed to save user to repository", "Failed to save user", err)
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
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch users from repository", "Failed to fetch users", err)
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
		HandleError(c, http.StatusNotFound, "NOT_FOUND", "User not found in repository", "User not found", err)
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
		HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to bind update user role request", "Role is required", err)
		return
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), req.Role)
	if err != nil || role == nil {
		HandleError(c, http.StatusBadRequest, "INVALID_ROLE", "Role not found in repository", "Role does not exist", err)
		return
	}

	err = h.userRepo.SetUserAppRole(c.Request.Context(), userID, h.appID, req.Role)
	if err != nil {
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update user app role in repository", "Failed to update user role", err)
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
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete user from Firebase", "Failed to delete user from Firebase", err)
		return
	}

	err := h.userRepo.Delete(c.Request.Context(), userID)
	if err != nil {
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete user from database", "Failed to delete user from database", err)
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
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch roles from repository", "Failed to fetch roles", err)
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
		HandleError(c, http.StatusNotFound, "NOT_FOUND", "Role not found in repository", "Role not found", err)
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
		HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "Role ID is missing", "Role ID is required", nil)
		return
	}

	err := h.roleRepo.Create(c.Request.Context(), &role)
	if err != nil {
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create role in repository", "Failed to create role", err)
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
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update role in repository", "Failed to update role", err)
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
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete role from repository", "Failed to delete role", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"message": "Role deleted"},
	})
}
