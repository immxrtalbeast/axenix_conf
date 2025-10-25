package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/immxrtalbeast/axenix_conf/internal/service"
)

type UserController struct {
	users service.UserInteractor
}

func NewUserController(users service.UserInteractor) *UserController {
	return &UserController{users: users}
}

func (c *UserController) CreateUser(ctx *gin.Context) {
	type request struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email"`
	}

	var req request
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	user, err := c.users.CreateUser(ctx.Request.Context(), req.Name, req.Email)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"user": user})
}

func (c *UserController) GetUser(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("userID"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := c.users.GetUser(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"user": user})
}
