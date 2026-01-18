package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"dynamic-route-53-dns/internal/api"
	"dynamic-route-53-dns/internal/database"
	"dynamic-route-53-dns/internal/route53"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	fiberadapter "github.com/awslabs/aws-lambda-go-api-proxy/fiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

//go:embed templates
var templatesFS embed.FS

var fiberLambda *fiberadapter.FiberLambda

func initAWS() {
	// Initialize database
	if err := database.Init(context.Background()); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Route 53 client
	if err := route53.Init(context.Background()); err != nil {
		log.Fatalf("Failed to initialize Route 53 client: %v", err)
	}
}

func init() {
	// Only initialize AWS clients in Lambda environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		initAWS()
		// Create Fiber app
		app := createApp()
		// Create Lambda adapter
		fiberLambda = fiberadapter.New(app)
	}
}

func createApp() *fiber.App {
	// Get templates subdirectory
	templatesSubFS, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		log.Fatalf("Failed to get templates subdirectory: %v", err)
	}

	// Configure Fiber with embedded templates
	engine := NewHTMLEngine(templatesSubFS)

	app := fiber.New(fiber.Config{
		Views:                   engine,
		DisableStartupMessage:   true,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"*"},
		ProxyHeader:             "X-Forwarded-For",
	})

	// Recovery middleware
	app.Use(recover.New())

	// Setup routes
	api.SetupRoutes(app)

	return app
}

// Handler is the Lambda handler function for HTTP API v2
func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return fiberLambda.ProxyWithContextV2(ctx, req)
}

func main() {
	// Check if running in Lambda
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(Handler)
	} else {
		// Local development mode - initialize AWS clients
		initAWS()
		app := createApp()
		log.Println("Starting server on :3000")
		if err := app.Listen(":3000"); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}
}
