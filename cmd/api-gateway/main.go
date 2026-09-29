// Command api-gateway is the single public entry point: it serves the SPA and
// proxies /api/v1/* to internal services.
package main

import (
	"log"

	"github.com/trezutil/trezutil/internal/gateway"
	"github.com/trezutil/trezutil/packages/common"
)

func main() {
	logger := common.Logger("api-gateway")
	cfg := gateway.LoadConfig()
	gw := gateway.New(cfg, logger)

	srv := common.NewServer(cfg.Addr, gw.Handler())
	// Proxied responses (e.g. search) may take longer than the default write timeout.
	srv.WriteTimeout = cfg.UpstreamTimeout + 5e9
	if err := common.Run(srv, logger, "api-gateway"); err != nil {
		log.Fatalf("api-gateway: %v", err)
	}
}
