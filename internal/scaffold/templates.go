package scaffold

// projectFiles returns a map of file path -> template string for the new project.
func projectFiles(data templateData) map[string]string {
	return map[string]string{
		"go.mod":             tmplGoMod,
		"main.go":            tmplMainGo,
		".env.example":       tmplEnvExample,
		".gitignore":         tmplGitignore,
		"Dockerfile":         tmplDockerfile,
		"docker-compose.yml": tmplDockerCompose,
		"Makefile":           tmplMakefile,
		"README.md":          tmplReadme,

		// global
		"global/configuration.go": tmplGlobalConfiguration,
		"global/errors.go":        tmplGlobalErrors,
		"global/global.go":        tmplGlobalGlobal,

		// middlewares
		"middlewares/middleware.go": tmplMiddleware,

		// models
		"models/dto.go": tmplModelsDTO,

		// utils
		"utils/common/common.go": tmplUtilsCommon,
		"utils/common/uuid.go":   tmplUtilsUUID,
		"utils/token/token.go":   tmplUtilsToken,
		"utils/validator/validator.go": tmplUtilsValidator,

		// mysql sqitch
		"mysql/sqitch.conf": tmplSqitchConf,
		"mysql/sqitch.plan": tmplSqitchPlan,

		// example module
		"example/interface.go":                              tmplExampleInterface,
		"example/handler/http/example_handler.go":           tmplExampleHandler,
		"example/repository/mysql/example_repository.go":    tmplExampleRepository,
		"example/service/example_service.go":                tmplExampleService,
		"example/_mock/example_repository_interface.go":     tmplExampleMockRepo,
		"example/_mock/example_service_interface.go":        tmplExampleMockService,
	}
}

// ── go.mod ──────────────────────────────────────────────────────────────────

var tmplGoMod = `module {{.ModuleName}}

go 1.23.0

require (
	github.com/go-playground/validator/v10 v10.27.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/labstack/echo/v4 v4.13.3
	github.com/spf13/viper v1.20.1
	gorm.io/driver/mysql v1.5.7
	gorm.io/gorm v1.25.12
)
`

// ── main.go ─────────────────────────────────────────────────────────────────

var tmplMainGo = `package main

import (
	"fmt"
	"net/http"
	"os"

	exampleHTTPHandler "{{.ModuleName}}/example/handler/http"
	exampleRepository "{{.ModuleName}}/example/repository/mysql"
	exampleService "{{.ModuleName}}/example/service"

	"{{.ModuleName}}/global"
	"{{.ModuleName}}/middlewares"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	var configuration global.Configuration

	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("No .env file found, using environment variables")
	}

	viper.AutomaticEnv()
	viper.SetDefault("host_port", getEnv("APP_PORT", "9000"))
	viper.SetDefault("db_host", getEnv("DB_HOST", "localhost"))
	viper.SetDefault("db_port", getEnv("DB_PORT", "3306"))
	viper.SetDefault("db_name", getEnv("DB_NAME", "{{.PackageName}}"))
	viper.SetDefault("db_user", getEnv("DB_USER", "root"))
	viper.SetDefault("db_pass", getEnv("DB_PASSWORD", ""))
	viper.SetDefault("private_jwt_access_token_secret", getEnv("JWT_SECRET", "default-secret-key"))
	viper.SetDefault("private_jwt_refresh_token_secret", getEnv("JWT_REFRESH_SECRET", "default-refresh-secret-key"))

	if err := viper.Unmarshal(&configuration); err != nil {
		panic("Unable to decode configuration into struct")
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		configuration.DbUser,
		configuration.DbPass,
		configuration.DbHost,
		configuration.DbPort,
		configuration.DbName,
	)
	mysqlDb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Error connecting to database")
	}

	e := echo.New()
	mw := middlewares.InitMiddleware([]byte(configuration.PrivateJWTAccessTokenSecret))
	e.Use(mw.ValidateCORS)

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{"message": "{{.ProjectName}} is live"})
	})

	// Initialize example module
	exampleRepo := exampleRepository.NewExampleMySQLRepository(mysqlDb)
	exampleSvc := exampleService.NewExampleService(exampleRepo)
	exampleHTTPHandler.NewExampleHandler(e, exampleSvc, mw)

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", configuration.HostPort)))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
`

// ── .env.example ────────────────────────────────────────────────────────────

var tmplEnvExample = `HOST_URL=127.0.0.1
HOST_PORT=9000
DB_HOST=localhost
DB_NAME={{.PackageName}}
DB_USER=root
DB_PASS=
DB_PORT=3306
TIMEOUT_DURATION=30
PRIVATE_JWT_ACCESS_TOKEN_SECRET=your-secret-key
PRIVATE_JWT_REFRESH_TOKEN_SECRET=your-refresh-secret-key
`

// ── .gitignore ───────────────────────────────────────────────────────────────

var tmplGitignore = `.env
*.exe
*.out
vendor/
{{.ProjectName}}
`

// ── Dockerfile ───────────────────────────────────────────────────────────────

var tmplDockerfile = `# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o {{.ProjectName}} .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /root/

COPY --from=builder /app/{{.ProjectName}} .

EXPOSE 9000

CMD ["./{{.ProjectName}}"]
`

// ── docker-compose.yml ───────────────────────────────────────────────────────

var tmplDockerCompose = `version: "3.8"

services:
  app:
    build: .
    ports:
      - "9000:9000"
    env_file:
      - .env
    depends_on:
      - mysql

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: rootpassword
      MYSQL_DATABASE: {{.PackageName}}
      MYSQL_USER: appuser
      MYSQL_PASSWORD: apppassword
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
      - ./mysql/init:/docker-entrypoint-initdb.d

volumes:
  mysql_data:
`

// ── Makefile ─────────────────────────────────────────────────────────────────

var tmplMakefile = `.PHONY: run build test tidy docker-up docker-down

run:
	go run main.go

build:
	CGO_ENABLED=0 GOOS=linux go build -o {{.ProjectName}} .

test:
	go test ./...

tidy:
	go mod tidy

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down
`

// ── README.md ────────────────────────────────────────────────────────────────

var tmplReadme = `# {{.ProjectName}}

A Go service built with clean architecture.

## Getting Started

` + "```bash" + `
cp .env.example .env   # configure your environment
go mod tidy
go run main.go
` + "```" + `

## Project Structure

` + "```" + `
{{.ProjectName}}/
├── main.go
├── global/          # shared config, errors, response types
├── middlewares/     # echo middleware (CORS, JWT)
├── models/          # shared DTOs and DB models
├── utils/           # helpers: common, token, validator
├── mysql/           # database migration scripts (sqitch)
└── example/         # sample module — copy to add new features
    ├── interface.go
    ├── _mock/
    ├── handler/http/
    ├── repository/mysql/
    └── service/
` + "```" + `

## Make Commands

| Command | Description |
|---------|-------------|
| ` + "`make run`" + ` | Run the server |
| ` + "`make build`" + ` | Build binary |
| ` + "`make test`" + ` | Run all tests |
| ` + "`make tidy`" + ` | Tidy Go modules |
| ` + "`make docker-up`" + ` | Start with docker-compose |
| ` + "`make docker-down`" + ` | Stop docker-compose |
`

// ── global/configuration.go ──────────────────────────────────────────────────

var tmplGlobalConfiguration = `package global

type Configuration struct {
	HostUrl                      string ` + "`" + `mapstructure:"host_url"` + "`" + `
	HostPort                     string ` + "`" + `mapstructure:"host_port"` + "`" + `
	DbHost                       string ` + "`" + `mapstructure:"db_host"` + "`" + `
	DbName                       string ` + "`" + `mapstructure:"db_name"` + "`" + `
	DbUser                       string ` + "`" + `mapstructure:"db_user"` + "`" + `
	DbPass                       string ` + "`" + `mapstructure:"db_pass"` + "`" + `
	DbPort                       string ` + "`" + `mapstructure:"db_port"` + "`" + `
	TimeoutDuration              int    ` + "`" + `mapstructure:"timeout_duration"` + "`" + `
	PrivateJWTAccessTokenSecret  string ` + "`" + `mapstructure:"private_jwt_access_token_secret"` + "`" + `
	PrivateJWTRefreshTokenSecret string ` + "`" + `mapstructure:"private_jwt_refresh_token_secret"` + "`" + `
}
`

// ── global/errors.go ─────────────────────────────────────────────────────────

var tmplGlobalErrors = `package global

import "errors"

var (
	ErrInternalServer = errors.New("internal server error")
	ErrNotFound       = errors.New("item not found")
	ErrConflict       = errors.New("item already exists")
	ErrBadInput       = errors.New("invalid input")
)

type BadResponse struct {
	Code    int    ` + "`" + `json:"code"` + "`" + `
	Message string ` + "`" + `json:"message"` + "`" + `
}

type SuccessResponse struct {
	Code    int         ` + "`" + `json:"code"` + "`" + `
	Message string      ` + "`" + `json:"message"` + "`" + `
	Data    interface{} ` + "`" + `json:"data"` + "`" + `
}
`

// ── global/global.go ─────────────────────────────────────────────────────────

var tmplGlobalGlobal = `package global

import "time"

type PostgresDefault struct {
	UpdatedAt time.Time ` + "`" + `json:"updated_at,omitempty"` + "`" + `
	CreatedAt time.Time ` + "`" + `json:"created_at,omitempty"` + "`" + `
	DeletedAt time.Time ` + "`" + `json:"deleted_at,omitempty"` + "`" + `
	UpdatedBy string    ` + "`" + `json:"updated_by,omitempty"` + "`" + `
	DeletedBy string    ` + "`" + `json:"deleted_by,omitempty"` + "`" + `
}

type ResponseError struct {
	Message string ` + "`" + `json:"message"` + "`" + `
}

type ResponseSuccess struct {
	Message string ` + "`" + `json:"message"` + "`" + `
}

type TokenResponse struct {
	AccessToken  string    ` + "`" + `json:"access_token"` + "`" + `
	RefreshToken string    ` + "`" + `json:"refresh_token"` + "`" + `
	ExpiredAt    time.Time ` + "`" + `json:"expired_at"` + "`" + `
}
`

// ── middlewares/middleware.go ─────────────────────────────────────────────────

var tmplMiddleware = `package middlewares

import (
	"net/http"
	"strings"

	"{{.ModuleName}}/global"
	"{{.ModuleName}}/utils/token"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

type GoMiddlewareInterface interface {
	ValidateCORS(next echo.HandlerFunc) echo.HandlerFunc
	ValidateToken(next echo.HandlerFunc) echo.HandlerFunc
}

type GoMiddleware struct {
	AccessSecret []byte
}

func InitMiddleware(accessSecret []byte) GoMiddlewareInterface {
	return &GoMiddleware{AccessSecret: accessSecret}
}

func (gm *GoMiddleware) ValidateCORS(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Access-Control-Allow-Origin", "*")
		return next(c)
	}
}

func (gm *GoMiddleware) ValidateToken(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return c.JSON(http.StatusUnauthorized, global.BadResponse{
				Code:    http.StatusUnauthorized,
				Message: "Authorization header is missing",
			})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.JSON(http.StatusUnauthorized, global.BadResponse{
				Code:    http.StatusUnauthorized,
				Message: "Invalid Authorization header format",
			})
		}

		accessToken := parts[1]
		claims := &token.Claims{}
		parsedToken, err := jwt.ParseWithClaims(accessToken, claims, func(t *jwt.Token) (interface{}, error) {
			return gm.AccessSecret, nil
		})

		if err != nil || !parsedToken.Valid {
			return c.JSON(http.StatusUnauthorized, global.BadResponse{
				Code:    http.StatusUnauthorized,
				Message: "Invalid access token",
			})
		}
		return next(c)
	}
}
`

// ── models/dto.go ────────────────────────────────────────────────────────────

var tmplModelsDTO = `package models

import "time"

// ExampleRequest is the request payload for creating an example resource.
type ExampleRequest struct {
	Name        string ` + "`" + `json:"name" validate:"required"` + "`" + `
	Description string ` + "`" + `json:"description"` + "`" + `
}

// ExampleResponse is the response payload for an example resource.
type ExampleResponse struct {
	ID          string    ` + "`" + `json:"id"` + "`" + `
	Name        string    ` + "`" + `json:"name"` + "`" + `
	Description string    ` + "`" + `json:"description"` + "`" + `
	CreatedAt   time.Time ` + "`" + `json:"created_at"` + "`" + `
}

// Example is the database model for the example resource.
type Example struct {
	ID          string    ` + "`" + `gorm:"primaryKey;type:varchar(36)" json:"id"` + "`" + `
	Name        string    ` + "`" + `gorm:"type:varchar(255);not null" json:"name"` + "`" + `
	Description string    ` + "`" + `gorm:"type:text" json:"description"` + "`" + `
	CreatedBy   string    ` + "`" + `gorm:"type:varchar(100)" json:"created_by"` + "`" + `
	UpdatedBy   string    ` + "`" + `gorm:"type:varchar(100)" json:"updated_by"` + "`" + `
	CreatedAt   time.Time ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt   time.Time ` + "`" + `json:"updated_at"` + "`" + `
}

func (Example) TableName() string {
	return "examples"
}
`

// ── utils/common/common.go ───────────────────────────────────────────────────

var tmplUtilsCommon = `package common

import "strings"

// ToSnakeCase converts a camelCase string to snake_case.
func ToSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
`

// ── utils/common/uuid.go ─────────────────────────────────────────────────────

var tmplUtilsUUID = `package common

import "github.com/google/uuid"

// NewUUID generates a new UUID string.
func NewUUID() string {
	return uuid.New().String()
}

// IsValidUUID returns true if the given string is a valid UUID.
func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}
`

// ── utils/token/token.go ─────────────────────────────────────────────────────

var tmplUtilsToken = `package token

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims defines the JWT claims structure.
type Claims struct {
	UserID string ` + "`" + `json:"user_id"` + "`" + `
	Email  string ` + "`" + `json:"email"` + "`" + `
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a signed JWT access token.
func GenerateAccessToken(userID, email string, secret []byte, expiry time.Duration) (string, time.Time, error) {
	expiredAt := time.Now().Add(expiry)
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiredAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(secret)
	return signed, expiredAt, err
}

// ParseToken parses and validates a JWT token string.
func ParseToken(tokenStr string, secret []byte) (*Claims, error) {
	claims := &Claims{}
	t, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil || !t.Valid {
		return nil, err
	}
	return claims, nil
}
`

// ── utils/validator/validator.go ─────────────────────────────────────────────

var tmplUtilsValidator = `package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateStruct validates a struct using its validate tags.
func ValidateStruct(s interface{}) error {
	if err := validate.Struct(s); err != nil {
		var errs []string
		for _, e := range err.(validator.ValidationErrors) {
			errs = append(errs, fmt.Sprintf("field %s failed on '%s' rule", e.Field(), e.Tag()))
		}
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
`

// ── mysql/sqitch.conf ────────────────────────────────────────────────────────

var tmplSqitchConf = `[core]
	engine = mysql
	plan_file = sqitch.plan
	top_dir = .

[engine "mysql"]
	target = db:mysql://root@localhost/{{.PackageName}}
`

// ── mysql/sqitch.plan ────────────────────────────────────────────────────────

var tmplSqitchPlan = `%syntax-version=1.0.0
%project={{.PackageName}}
%uri=https://github.com/yourusername/{{.ProjectName}}

`

// ── example/interface.go ─────────────────────────────────────────────────────

var tmplExampleInterface = `package example

import (
	"{{.ModuleName}}/models"
	"context"
)

// ExampleRepositoryInterface defines the database operations for the example resource.
type ExampleRepositoryInterface interface {
	Create(ctx context.Context, example *models.Example) error
	GetByID(ctx context.Context, id string) (*models.Example, error)
	List(ctx context.Context) ([]*models.Example, error)
}

// ExampleServiceInterface defines the business logic for the example resource.
type ExampleServiceInterface interface {
	Create(ctx context.Context, req *models.ExampleRequest) (*models.ExampleResponse, error)
	GetByID(ctx context.Context, id string) (*models.ExampleResponse, error)
	List(ctx context.Context) ([]*models.ExampleResponse, error)
}
`

// ── example/handler/http/example_handler.go ──────────────────────────────────

var tmplExampleHandler = `package http

import (
	"net/http"

	"{{.ModuleName}}/example"
	"{{.ModuleName}}/global"
	"{{.ModuleName}}/middlewares"
	"{{.ModuleName}}/models"
	"{{.ModuleName}}/utils/validator"

	"github.com/labstack/echo/v4"
)

type ExampleHandler struct {
	exampleService example.ExampleServiceInterface
	middleware     middlewares.GoMiddlewareInterface
}

// NewExampleHandler registers routes for the example module.
func NewExampleHandler(e *echo.Echo, svc example.ExampleServiceInterface, mw middlewares.GoMiddlewareInterface) {
	h := &ExampleHandler{exampleService: svc, middleware: mw}

	v1 := e.Group("/v1")
	v1.POST("/examples", h.Create)
	v1.GET("/examples/:id", h.GetByID)
	v1.GET("/examples", h.List)
}

func (h *ExampleHandler) Create(c echo.Context) error {
	var req models.ExampleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, global.BadResponse{Code: http.StatusBadRequest, Message: "invalid request body"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, global.BadResponse{Code: http.StatusBadRequest, Message: err.Error()})
	}

	resp, err := h.exampleService.Create(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, global.BadResponse{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, global.SuccessResponse{Code: http.StatusCreated, Message: "created", Data: resp})
}

func (h *ExampleHandler) GetByID(c echo.Context) error {
	id := c.Param("id")
	resp, err := h.exampleService.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, global.BadResponse{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	if resp == nil {
		return c.JSON(http.StatusNotFound, global.BadResponse{Code: http.StatusNotFound, Message: "not found"})
	}
	return c.JSON(http.StatusOK, global.SuccessResponse{Code: http.StatusOK, Message: "ok", Data: resp})
}

func (h *ExampleHandler) List(c echo.Context) error {
	resp, err := h.exampleService.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, global.BadResponse{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	return c.JSON(http.StatusOK, global.SuccessResponse{Code: http.StatusOK, Message: "ok", Data: resp})
}
`

// ── example/repository/mysql/example_repository.go ───────────────────────────

var tmplExampleRepository = `package mysql

import (
	"context"
	"errors"

	"{{.ModuleName}}/example"
	"{{.ModuleName}}/models"

	"gorm.io/gorm"
)

type exampleMySQLRepository struct {
	db *gorm.DB
}

// NewExampleMySQLRepository creates a new example repository instance.
func NewExampleMySQLRepository(db *gorm.DB) example.ExampleRepositoryInterface {
	return &exampleMySQLRepository{db: db}
}

func (r *exampleMySQLRepository) Create(ctx context.Context, e *models.Example) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *exampleMySQLRepository) GetByID(ctx context.Context, id string) (*models.Example, error) {
	var e models.Example
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func (r *exampleMySQLRepository) List(ctx context.Context) ([]*models.Example, error) {
	var list []*models.Example
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}
`

// ── example/service/example_service.go ───────────────────────────────────────

var tmplExampleService = `package service

import (
	"context"
	"fmt"

	"{{.ModuleName}}/example"
	"{{.ModuleName}}/models"
	"{{.ModuleName}}/utils/common"
)

type exampleService struct {
	repo example.ExampleRepositoryInterface
}

// NewExampleService creates a new example service instance.
func NewExampleService(repo example.ExampleRepositoryInterface) example.ExampleServiceInterface {
	return &exampleService{repo: repo}
}

func (s *exampleService) Create(ctx context.Context, req *models.ExampleRequest) (*models.ExampleResponse, error) {
	e := &models.Example{
		ID:          common.NewUUID(),
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   "system",
		UpdatedBy:   "system",
	}
	if err := s.repo.Create(ctx, e); err != nil {
		return nil, fmt.Errorf("failed to create example: %w", err)
	}
	return &models.ExampleResponse{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		CreatedAt:   e.CreatedAt,
	}, nil
}

func (s *exampleService) GetByID(ctx context.Context, id string) (*models.ExampleResponse, error) {
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get example: %w", err)
	}
	if e == nil {
		return nil, nil
	}
	return &models.ExampleResponse{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		CreatedAt:   e.CreatedAt,
	}, nil
}

func (s *exampleService) List(ctx context.Context) ([]*models.ExampleResponse, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list examples: %w", err)
	}
	result := make([]*models.ExampleResponse, 0, len(list))
	for _, e := range list {
		result = append(result, &models.ExampleResponse{
			ID:          e.ID,
			Name:        e.Name,
			Description: e.Description,
			CreatedAt:   e.CreatedAt,
		})
	}
	return result, nil
}
`

// ── example/_mock/example_repository_interface.go ────────────────────────────

var tmplExampleMockRepo = `package mock

import (
	"context"

	"{{.ModuleName}}/models"

	"github.com/stretchr/testify/mock"
)

type MockExampleRepository struct {
	mock.Mock
}

func (m *MockExampleRepository) Create(ctx context.Context, e *models.Example) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}

func (m *MockExampleRepository) GetByID(ctx context.Context, id string) (*models.Example, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Example), args.Error(1)
}

func (m *MockExampleRepository) List(ctx context.Context) ([]*models.Example, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Example), args.Error(1)
}
`

// ── example/_mock/example_service_interface.go ───────────────────────────────

var tmplExampleMockService = `package mock

import (
	"context"

	"{{.ModuleName}}/models"

	"github.com/stretchr/testify/mock"
)

type MockExampleService struct {
	mock.Mock
}

func (m *MockExampleService) Create(ctx context.Context, req *models.ExampleRequest) (*models.ExampleResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ExampleResponse), args.Error(1)
}

func (m *MockExampleService) GetByID(ctx context.Context, id string) (*models.ExampleResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ExampleResponse), args.Error(1)
}

func (m *MockExampleService) List(ctx context.Context) ([]*models.ExampleResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.ExampleResponse), args.Error(1)
}
`
