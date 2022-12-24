package routes

import (
	"os"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	databse "github.com/shahriarsohan/url-shortner-go-fiber/database"
	"github.com/shahriarsohan/url-shortner-go-fiber/helpers"
)

type request struct {
	URL         string        `json:"url"`
	CustomShort string        `json:"customshort"`
	Expiry      time.Duration `json:"expiry"`
}

type response struct {
	URL            string        `json:"url"`
	CustomShort    string        `json:"customshort"`
	Expiry         string        `json:"expiry"`
	XRateRemaining int           `json:"xratereamining"`
	XRateLimitRest time.Duration `json:"xratelimitrest"`
}

func ShortenURL(c *fiber.Ctx) error {
	body := new(request)

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot parse JSON"})
	}

	//implement rate limiting
	r2 := databse.CreateClient(1)
	defer r2.Close()
	val, err := r2.Get(databse.Ctx, c.IP()).Result()
	if err == redis.Nil {
		_ = r2.Set(databse.Ctx, c.IP(), os.Getenv("API_QOUTA"), 30*60*time.Second).Err()
	} else {
		val, _ := r2.Get(databse.Ctx, c.IP()).Result()
		valInt, _ := strconv.Atoi(val)
		if valInt <= 0 {
			limit, _ := r2.TTL(databse.Ctx, c.IP()).Result()
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error":           "limit exceeded",
				"rate_limit_rest": limit / time.Nanosecond / time.Minute,
			})
		}
	}

	// check if the input if an actual URL
	if !govalidator.IsURL(body.URL) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL"})
	}

	//check for domain error
	if !helpers.RemoveDomainError(body.URL) {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Invalid Request"})
	}

	//enfoce https , SSL
	body.URL = helpers.EnforceHttp(body.URL)

	var id string
	if body.CustomShort == "" {
		id = uuid.New().String()[:6]
	} else {
		id = body.CustomShort
	}

	r := databse.CreateClient(0)
	defer r.Close()

	val, _ = r.Get(databse.Ctx, id).Result()
	if val != "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "URL costom short is already in user",
		})
	}

	if body.Expiry == 0 {
		body.Expiry = 24
	}

	err = r.Set(databse.Ctx, id, body.URL, body.Expiry*3600*time.Second).Err()

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to collect server",
		})
	}

	resp := response{
		URL:            body.URL,
		CustomShort:    "",
		Expiry:         body.Expiry.String(),
		XRateRemaining: 10,
		XRateLimitRest: 30,
	}

	r2.Decr(databse.Ctx, c.IP())

	val, _ = r2.Get(databse.Ctx, c.IP()).Result()
	resp.XRateRemaining, _ = strconv.Atoi(val)

	ttl, _ := r2.TTL(databse.Ctx, c.IP()).Result()

	resp.XRateLimitRest = ttl / time.Nanosecond / time.Minute

	resp.CustomShort = os.Getenv("DOMAIN") + "/" + id

	return c.Status(fiber.StatusOK).JSON(resp)

}
