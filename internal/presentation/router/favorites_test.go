package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/token"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

type favoritesCatalogueRepo struct {
	repository.CatalogueRepository
	listedFor []string
}

func (r *favoritesCatalogueRepo) Favorites(_ context.Context, search model.CatalogueFavoritesSearch) ([]model.CatalogueOffer, error) {
	r.listedFor = append(r.listedFor, search.UserPublicID)
	return nil, nil
}

func TestFavoriteListUsesBearerIdentity(t *testing.T) {
	log := logger.New("debug", &bytes.Buffer{})
	sessions := service.NewSessionService(memory.NewUserRepository(), token.NewJWTManager(rbacSecret, rbacIssuer, time.Hour), nil, log)
	session, err := sessions.Login(context.Background(), testPhone)
	if err != nil {
		t.Fatal(err)
	}
	repo := &favoritesCatalogueRepo{}
	devAuth := middleware.NewDevUserAuth(func(context.Context, int64) (bool, error) { return true, nil }, true)
	r, err := router.New(&bootstrap.Config{}, log, middleware.NewErrorHandler(log), middleware.NewAuthMiddleware(sessions), nil, router.Handlers{
		Health:    handler.NewHealthHandler(service.NewHealthService("test", time.Second, log)),
		Auth:      handler.NewAuthHandler(nil, nil),
		Catalogue: handler.NewCatalogueHandler(service.NewCatalogueService(repo)),
		UserAuth:  devAuth,
	})
	if err != nil {
		t.Fatal(err)
	}

	list := func(authHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/favorites/offers", nil)
		req.Header.Set(middleware.DevUserHeader, "42")
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := list(""); w.Code != http.StatusUnauthorized {
		t.Fatalf("dev header alone: got %d, want 401", w.Code)
	}
	w := list("Bearer " + session.AccessToken)
	var page dto.CatalogueOfferList
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &page) != nil || page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("authorized empty list: %d %s", w.Code, w.Body.String())
	}
	if len(repo.listedFor) != 1 || repo.listedFor[0] != session.User.ID {
		t.Fatalf("looked up favorites for %v, want only %q", repo.listedFor, session.User.ID)
	}
}
