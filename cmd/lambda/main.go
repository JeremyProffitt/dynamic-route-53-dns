package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	fiberadapter "github.com/awslabs/aws-lambda-go-api-proxy/fiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"

	"github.com/dynamic-route-53-dns/internal/api"
	"github.com/dynamic-route-53-dns/internal/api/handlers"
	"github.com/dynamic-route-53-dns/internal/api/middleware"
	"github.com/dynamic-route-53-dns/internal/database"
	"github.com/dynamic-route-53-dns/internal/route53"
	"github.com/dynamic-route-53-dns/internal/service"
)

var (
	// Database client
	dbClient *database.Client

	// Route 53 client
	r53Client *route53.Route53Client

	// Services
	authService   *service.AuthService
	zoneService   *service.ZoneService
	ddnsService   *service.DDNSService
	updateService *service.UpdateService

	// Handlers
	authHandler   *handlers.AuthHandler
	zoneHandler   *handlers.ZoneHandler
	ddnsHandler   *handlers.DDNSHandler
	updateHandler *handlers.UpdateHandler

	// Middleware
	rateLimitMiddleware *middleware.RateLimitMiddleware

	// Fiber app
	app *fiber.App

	// Lambda adapter
	fiberLambda *fiberadapter.FiberLambda
)

func init() {
	ctx := context.Background()

	// Get table name from environment
	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		log.Fatal("DYNAMODB_TABLE environment variable is required")
	}

	// Initialize database client
	var err error
	dbClient, err = database.NewClient(ctx, tableName)
	if err != nil {
		log.Fatalf("Failed to initialize database client: %v", err)
	}

	// Initialize Route 53 client
	r53Client, err = route53.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Route 53 client: %v", err)
	}

	// Initialize services
	authService = service.NewAuthService(dbClient)
	zoneService = service.NewZoneService(r53Client)
	ddnsService = service.NewDDNSService(dbClient, r53Client)
	updateService = service.NewUpdateService(dbClient, r53Client)

	// Initialize handlers
	authHandler = handlers.NewAuthHandler(authService)
	zoneHandler = handlers.NewZoneHandler(zoneService)
	ddnsHandler = handlers.NewDDNSHandler(ddnsService, zoneService)
	updateHandler = handlers.NewUpdateHandler(updateService)

	// Initialize middleware
	rateLimitMiddleware = middleware.NewRateLimitMiddleware(dbClient)

	// Create HTML template engine
	engine := html.New("./web/templates", ".html")

	// Create Fiber app with configuration
	app = fiber.New(fiber.Config{
		Views:       engine,
		ViewsLayout: "layouts/base",
	})

	// Add recover middleware
	app.Use(recover.New())

	// Serve static files
	app.Static("/static", "./web/static")

	// Setup routes
	api.SetupRoutes(app, api.Handlers{
		Auth:   authHandler,
		Zone:   zoneHandler,
		DDNS:   ddnsHandler,
		Update: updateHandler,
	}, api.Middleware{
		RateLimit: rateLimitMiddleware,
	}, authService)

	// Create Lambda adapter
	fiberLambda = fiberadapter.New(app)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return fiberLambda.ProxyWithContext(ctx, req)
}

func main() {
	// Start Lambda handler
	lambda.Start(Handler)
}
