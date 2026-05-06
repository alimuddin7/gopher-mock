package main

import (
	"log"
	"strings"

	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/templui/templui/components"

	"gopher-mock/handler"
)

func main() {
	app := fiber.New()

	h := handler.NewMockHandler("configs.json")

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Requested-With, X-Auth-Token",
		AllowMethods: "GET, POST, PUT, DELETE, PATCH, OPTIONS",
	}))
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))
	app.Static("/static", "./static")
	app.Get("/templui/js/:filename", func(c *fiber.Ctx) error {
		filename := c.Params("filename")
		component := filename
		if idx := strings.Index(filename, "."); idx != -1 {
			component = filename[:idx]
		}
		path := component + "/" + filename
		file, err := components.TemplFiles.ReadFile(path)
		if err != nil {
			return c.Status(404).SendString("File not found")
		}
		c.Set("Content-Type", "application/javascript")
		return c.Send(file)
	})
	app.Use("/templui", filesystem.New(filesystem.Config{
		Root: http.FS(components.TemplFiles),
	}))
	app.Get("/", h.Index)
	app.Post("/save", h.Save)
	app.Post("/import-openapi", h.ImportOpenAPI)
	app.Post("/delete-config/:index", h.Delete)
	app.Post("/delete-configs", h.BulkDelete)
	app.All("/*", h.RequestResponseLogger(), h.Dynamic)

	log.Fatal(app.Listen(":3000"))
}
