package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/postgres/driver"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/seed"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
	"github.com/tahedig/too-good-to-go/backend/pkg/validation"
)

func main() {
	if err := run(); err != nil {
		logger.New("error", os.Stderr).Error("seed failed", "error", err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required to run seed")
	}

	log := logger.New(cfg.LogLevel, os.Stdout).With(
		"service", bootstrap.AppName,
		"env", cfg.AppEnv,
		"cmd", "seed",
	)

	seedCfg, err := buildSeedConfig(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := driver.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	summary, err := seed.Run(ctx, pool, seedCfg)
	if err != nil {
		return err
	}

	log.Info("seed completed",
		"admin_phone", validation.MaskPhone(seedCfg.AdminPhone),
		"vendors", summary.VendorUsers,
		"customers", summary.CustomerUsers,
		"stores", summary.Stores,
		"offers", summary.Offers,
		"orders", summary.Orders,
	)
	return nil
}

func buildSeedConfig(cfg *bootstrap.Config) (seed.Config, error) {
	adminPhone := strings.TrimSpace(os.Getenv("SEED_ADMIN_PHONE"))
	if adminPhone == "" && len(cfg.AdminPhones) > 0 {
		adminPhone = cfg.AdminPhones[0]
	}

	vendorPhones := splitCSV(os.Getenv("SEED_VENDOR_PHONES"))
	if len(vendorPhones) == 0 && len(cfg.VendorPhones) > 0 {
		vendorPhones = append(vendorPhones, cfg.VendorPhones...)
	}

	customerPhones := splitCSV(os.Getenv("SEED_CUSTOMER_PHONES"))

	// In local development, fall back to synthetic non-routable test numbers
	// when SEED_* / *_PHONES are not explicitly set.
	if cfg.AppEnv == bootstrap.EnvDevelopment {
		if adminPhone == "" {
			adminPhone = "+989000000001"
		}
		if len(vendorPhones) == 0 {
			vendorPhones = []string{"+989000000002", "+989000000003"}
		} else if len(vendorPhones) == 1 {
			vendorPhones = append(vendorPhones, "+989000000003")
		}
		if len(customerPhones) == 0 {
			customerPhones = []string{"+989000000004", "+989000000005"}
		} else if len(customerPhones) == 1 {
			customerPhones = append(customerPhones, "+989000000005")
		}
	}

	if adminPhone == "" {
		return seed.Config{}, fmt.Errorf("SEED_ADMIN_PHONE (or ADMIN_PHONES) must be set when APP_ENV=%s", cfg.AppEnv)
	}
	if len(vendorPhones) < 2 {
		return seed.Config{}, fmt.Errorf("SEED_VENDOR_PHONES must provide at least 2 phones when APP_ENV=%s", cfg.AppEnv)
	}
	if len(customerPhones) < 2 {
		return seed.Config{}, fmt.Errorf("SEED_CUSTOMER_PHONES must provide at least 2 phones when APP_ENV=%s", cfg.AppEnv)
	}

	return seed.Config{
		AppEnv:         cfg.AppEnv,
		AdminPhone:     adminPhone,
		VendorPhones:   vendorPhones,
		CustomerPhones: customerPhones,
	}, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
