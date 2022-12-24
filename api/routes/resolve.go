package routes

import (
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	databse "github.com/shahriarsohan/url-shortner-go-fiber/database"
)

func ResolveURL(c *fiber.Ctx) error {
	url := c.Params("url")

	r := databse.CreateClient(0)
	defer r.Close()

	value, err := r.Get(databse.Ctx, url).Result()
	if err == redis.Nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "short not found",
		})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "cannot connect to db",
		})
	}
	rInr := databse.CreateClient(1)
	defer rInr.Close()

	_ = rInr.Incr(databse.Ctx, "counter")
	return c.Redirect(value, 301)
}
