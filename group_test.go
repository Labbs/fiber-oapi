package fiberoapi

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestGroupFunctionality(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Test group creation
	v1 := oapi.Group("/api/v1")
	assert.NotNil(t, v1)
	assert.Equal(t, "/api/v1", v1.GetPrefix())
	assert.Equal(t, oapi, v1.GetApp())

	// Test sub-group creation
	admin := v1.Group("/admin")
	assert.NotNil(t, admin)
	assert.Equal(t, "/api/v1/admin", admin.GetPrefix())
	assert.Equal(t, oapi, admin.GetApp())

	// Test that operations are registered correctly
	initialOpCount := len(oapi.GetOperations())

	// Add a route to the group using the unified Get function
	Get(v1, "/users/:id", func(c *fiber.Ctx, input struct {
		ID int `path:"id"`
	}) (struct {
		ID int `json:"id"`
	}, struct{}) {
		return struct {
			ID int `json:"id"`
		}{ID: input.ID}, struct{}{}
	}, OpenAPIOptions{
		Summary: "Get user",
		Tags:    []string{"users"},
	})

	// Verify the operation was registered with the correct full path
	operations := oapi.GetOperations()
	assert.Equal(t, initialOpCount+1, len(operations))

	lastOp := operations[len(operations)-1]
	assert.Equal(t, "GET", lastOp.Method)
	assert.Equal(t, "/api/v1/users/:id", lastOp.Path)
	assert.Equal(t, "Get user", lastOp.Options.Summary)
	assert.Contains(t, lastOp.Options.Tags, "users")
}

func TestGroupWrapper(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	v1 := oapi.Group("/api/v1")

	// Test that the wrapper correctly registers operations
	Post(v1, "/users", func(c *fiber.Ctx, input struct {
		Name string `json:"name"`
	}) (struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}, struct{}) {
		return struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}{ID: 1, Name: input.Name}, struct{}{}
	}, OpenAPIOptions{
		Summary: "Create user",
		Tags:    []string{"users"},
	})

	// Verify the operation was registered correctly
	operations := oapi.GetOperations()
	assert.Greater(t, len(operations), 0)

	found := false
	for _, op := range operations {
		if op.Method == "POST" && op.Path == "/api/v1/users" {
			found = true
			assert.Equal(t, "Create user", op.Options.Summary)
			break
		}
	}
	assert.True(t, found, "POST /api/v1/users operation should be registered")
}

func TestNestedGroups(t *testing.T) {
	app := fiber.New()
	oapi := New(app)

	// Create nested groups
	api := oapi.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")

	assert.Equal(t, "/api", api.GetPrefix())
	assert.Equal(t, "/api/v1", v1.GetPrefix())
	assert.Equal(t, "/api/v1/users", users.GetPrefix())

	// Add an operation to the deeply nested group
	Get(users, "/:id", func(c *fiber.Ctx, input struct {
		ID int `path:"id"`
	}) (struct {
		ID int `json:"id"`
	}, struct{}) {
		return struct {
			ID int `json:"id"`
		}{ID: input.ID}, struct{}{}
	}, OpenAPIOptions{
		Summary: "Get user by ID",
		Tags:    []string{"users"},
	})

	// Verify the full path is correct
	operations := oapi.GetOperations()
	found := false
	for _, op := range operations {
		if op.Method == "GET" && op.Path == "/api/v1/users/:id" {
			found = true
			break
		}
	}
	assert.True(t, found, "GET /api/v1/users/:id operation should be registered")
}
