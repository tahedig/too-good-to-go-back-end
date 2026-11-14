// Package postgres implements the Postgres-backed repositories. The queries live in
// scripts/catalogue/sql/*.sql (tested by scripts/catalogue/test_catalogue_sql.sh) and are turned
// into catalogue_sql.go by scripts/catalogue/gen_sql_go.py.
package postgres

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// DB is satisfied by *pgxpool.Pool, *pgx.Conn and pgx.Tx.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type catalogueRepository struct{ db DB }

// NewCatalogueRepository creates a repository.CatalogueRepository.
func NewCatalogueRepository(db DB) repository.CatalogueRepository {
	return &catalogueRepository{db: db}
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// escapeLike makes user text literal inside ILIKE ... ESCAPE '\'.
func escapeLike(s string) string { return likeEscaper.Replace(s) }

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

// nilIfEmpty keeps empty filters NULL so the SQL's "$n IS NULL OR ..." short-circuits.
func nilIfEmptyInts(v []int16) any {
	if len(v) == 0 {
		return nil
	}
	return v
}

func nilIfEmptyStrings(v []string) any {
	if len(v) == 0 {
		return nil
	}
	return v
}

func nilIfBlank(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func cursorArgs(c *model.CatalogueCursor, withK2 bool) []any {
	if c == nil {
		if withK2 {
			return []any{nil, nil, nil}
		}
		return []any{nil, nil}
	}
	if withK2 {
		return []any{c.K1, c.K2, c.ID}
	}
	return []any{c.K1, c.ID}
}

// ---------------------------------------------------------------- categories & filters

func (r *catalogueRepository) Categories(ctx context.Context) ([]model.CatalogueCategory, error) {
	rows, err := r.db.Query(ctx, sqlHomeCategories)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.CatalogueCategory{}
	for rows.Next() {
		var c model.CatalogueCategory
		var id int16
		if err := rows.Scan(&id, &c.Slug, &c.NameFA, &c.Kind); err != nil {
			return nil, err
		}
		c.ID = int(id)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *catalogueRepository) FilterData(ctx context.Context) (model.CatalogueFilterData, error) {
	var d model.CatalogueFilterData
	rows, err := r.db.Query(ctx, sqlFilterAllergens)
	if err != nil {
		return d, err
	}
	d.Allergens = []model.CatalogueAllergen{}
	for rows.Next() {
		var a model.CatalogueAllergen
		if err := rows.Scan(&a.Code, &a.NameFA); err != nil {
			rows.Close()
			return d, err
		}
		d.Allergens = append(d.Allergens, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return d, err
	}
	err = r.db.QueryRow(ctx, sqlFilterPriceRange).Scan(&d.PriceMin, &d.PriceMax, &d.OfferCount)
	return d, err
}

// ---------------------------------------------------------------- catalogue & favorites

func scanCatalogueOffers(rows pgx.Rows) ([]model.CatalogueOffer, error) {
	defer rows.Close()
	out := []model.CatalogueOffer{}
	for rows.Next() {
		var (
			o         model.CatalogueOffer
			pubID     pgtype.UUID
			storePub  pgtype.UUID
			qty, disc int32
		)
		if err := rows.Scan(
			&pubID, &o.Title, &o.ImageURL, &o.Price, &o.OriginalPrice, &disc, &o.Currency,
			&qty, &o.PickupStart, &o.PickupEnd, &o.CategorySlug, &o.CategoryName,
			&storePub, &o.StoreName, &o.StoreLogoURL, &o.DistanceM, &o.SortKey, &o.ID, &o.IsFavorite,
		); err != nil {
			return nil, err
		}
		o.PublicID, o.StorePublicID = uuidString(pubID), uuidString(storePub)
		o.QuantityAvailable, o.DiscountPercent = int(qty), int(disc)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *catalogueRepository) Search(ctx context.Context, p model.CatalogueSearch) ([]model.CatalogueOffer, error) {
	var like any
	if q := strings.TrimSpace(p.Query); q != "" {
		like = "%" + escapeLike(q) + "%"
	}
	ck := cursorArgs(p.Cursor, false) // key, id
	// $1..$17, same order as scripts/catalogue/sql/catalogue_search.sql
	rows, err := r.db.Query(ctx, sqlCatalogueSearch,
		like, nilIfEmptyInts(p.CategoryIDs), nilIfBlank(p.Kind),
		p.PriceMin, p.PriceMax, p.PickupFrom, p.PickupTo,
		nilIfEmptyStrings(p.ExcludeAllergens), nilIfBlank(p.StorePublicID),
		p.Lat, p.Lng, p.Sort, ck[0], ck[1],
		p.UserID, p.Limit, p.RadiusKm,
	)
	if err != nil {
		return nil, err
	}
	return scanCatalogueOffers(rows)
}

func (r *catalogueRepository) Favorites(ctx context.Context, p model.CatalogueFavoritesSearch) ([]model.CatalogueOffer, error) {
	ck := cursorArgs(p.Cursor, false)
	// $1 user, $2 lat, $3 lng, $4 cursor key, $5 cursor id, $6 limit
	rows, err := r.db.Query(ctx, sqlFavoritesList, p.UserID, p.Lat, p.Lng, ck[0], ck[1], p.Limit)
	if err != nil {
		return nil, err
	}
	return scanCatalogueOffers(rows)
}

func (r *catalogueRepository) AddFavorite(ctx context.Context, userID int64, offerPublicID string) (bool, error) {
	var found bool
	err := r.db.QueryRow(ctx, sqlFavoriteAdd, userID, offerPublicID).Scan(&found)
	return found, err
}

func (r *catalogueRepository) RemoveFavorite(ctx context.Context, userID int64, offerPublicID string) error {
	_, err := r.db.Exec(ctx, sqlFavoriteRemove, userID, offerPublicID)
	return err
}

// ---------------------------------------------------------------- home sections

func (r *catalogueRepository) Home(ctx context.Context, section model.HomeSection, p model.HomeSearch) ([]model.HomeOffer, error) {
	var q string
	switch section {
	case model.HomeSectionToday:
		q = sqlHomeToday
	case model.HomeSectionPopularNearby:
		q = sqlHomePopularNearby
	case model.HomeSectionLastChance:
		q = sqlHomeLastChance
	default:
		return nil, pgx.ErrNoRows // unreachable: the service only passes the three constants
	}
	ck := cursorArgs(p.Cursor, true)
	// $1 limit, $2 lat, $3 lng, $4 radius km, $5..$7 cursor (k1, k2, id), $8 user
	rows, err := r.db.Query(ctx, q, p.Limit, p.Lat, p.Lng, p.RadiusKm, ck[0], ck[1], ck[2], p.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.HomeOffer{}
	for rows.Next() {
		var (
			o         model.HomeOffer
			pubID     pgtype.UUID
			storePub  pgtype.UUID
			qty, disc int32
		)
		if err := rows.Scan(
			&pubID, &o.Title, &o.ImageURL, &o.Price, &o.OriginalPrice, &disc, &o.Currency,
			&qty, &o.PickupStart, &o.PickupEnd, &storePub, &o.StoreName,
			&o.DistanceM, &o.K1, &o.K2, &o.ID, &o.IsFavorite,
		); err != nil {
			return nil, err
		}
		o.PublicID, o.StorePublicID = uuidString(pubID), uuidString(storePub)
		o.QuantityAvailable, o.DiscountPercent = int(qty), int(disc)
		out = append(out, o)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- map

func (r *catalogueRepository) Map(ctx context.Context, p model.CatalogueMapSearch) ([]model.MapOffer, error) {
	// $1 lat, $2 lng, $3 radius km, $4 category ids, $5 kind, $6 user, $7 limit
	rows, err := r.db.Query(ctx, sqlMapOffers,
		p.Lat, p.Lng, p.RadiusKm, nilIfEmptyInts(p.CategoryIDs), nilIfBlank(p.Kind), p.UserID, p.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MapOffer{}
	for rows.Next() {
		var (
			o         model.MapOffer
			pubID     pgtype.UUID
			storePub  pgtype.UUID
			qty, disc int32
		)
		if err := rows.Scan(
			&pubID, &o.Title, &o.ImageURL, &o.Price, &o.OriginalPrice, &disc, &o.Currency,
			&qty, &o.PickupStart, &o.PickupEnd, &o.CategorySlug, &storePub, &o.StoreName,
			&o.StoreLat, &o.StoreLng, &o.DistanceM, &o.IsFavorite,
		); err != nil {
			return nil, err
		}
		o.PublicID, o.StorePublicID = uuidString(pubID), uuidString(storePub)
		o.QuantityAvailable, o.DiscountPercent = int(qty), int(disc)
		out = append(out, o)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- users

func (r *catalogueRepository) UserExists(ctx context.Context, userID int64) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, sqlUserExists, userID).Scan(&ok)
	return ok, err
}
