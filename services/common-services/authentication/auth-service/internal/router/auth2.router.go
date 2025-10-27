// In router/oauth2_routes.go
package router

import (
	"auth-service/internal/handler"
	"fmt"
	"net/http"
	"x/shared/auth/middleware"

	"github.com/go-chi/chi/v5"
)

func SetupOAuth2Routes(r chi.Router, oauthHandler *handler.OAuth2Handler, auth *middleware.MiddlewareWithClient) {
	r.Route("/api/v1/oauth2", func(oauth chi.Router) {
		oauth.Get("/authorize", oauthHandler.Authorize) // Done
		oauth.Post("/token", oauthHandler.Token)
		oauth.Post("/revoke", oauthHandler.Revoke)
		oauth.Post("/introspect", oauthHandler.Introspect)

        // Auth required for consent and client management (temporary token)
        oauth.Get("/consent", oauthHandler.ShowConsent) // Done
        oauth.Get("/consent-ui", oauthHandler.ServeConsentUI) // Done

		oauth.Group(func(consent chi.Router) {
			consent.Use(auth.Require([]string{"main", "temp"}, nil, nil))
			consent.Post("/consent", oauthHandler.GrantConsent) // Done
		})


		oauth.Group(func(client chi.Router) {
			client.Use(auth.Require([]string{"main"}, nil, nil))
			client.Route("/clients", func(r chi.Router) {
				r.Post("/", oauthHandler.RegisterClient) // Done
				r.Get("/", oauthHandler.ListMyClients)
				r.Get("/{client_id}", oauthHandler.GetClient)
				r.Put("/{client_id}", oauthHandler.UpdateClient)
				r.Delete("/{client_id}", oauthHandler.DeleteClient)
				r.Post("/{client_id}/regenerate-secret", oauthHandler.RegenerateClientSecret)
			})
			client.Route("/consents", func(r chi.Router) {
				r.Get("/", oauthHandler.ListMyConsents) // Done
				r.Delete("/", oauthHandler.RevokeAllConsents) // Done
				r.Delete("/{client_id}", oauthHandler.RevokeConsent) // Done
			})
		})
	})
	// Debug: Print all registered routes
	fmt.Println("\n=== Registered OAuth2 Routes ===")
	chi.Walk(r, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		if len(route) > 0 {
			fmt.Printf("%s %s\n", method, route)
		}
		return nil
	})
	fmt.Println("================================")
}
