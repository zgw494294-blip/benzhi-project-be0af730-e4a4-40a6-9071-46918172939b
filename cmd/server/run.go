package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subtitle-review/internal/store"
	"subtitle-review/internal/web"
	"subtitle-review/internal/workflow"
)

func run(cfg config, logger *log.Logger) error {
	dataDir := cfg.dataDir
	if cfg.selfcheck {
		tmp, err := os.MkdirTemp("", "subtitle-review-selfcheck-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		dataDir = tmp
	}
	repo, err := store.Open(dataDir)
	if err != nil {
		return fmt.Errorf("打开数据仓储: %w", err)
	}
	defer repo.Close()
	service := workflow.NewService(repo)
	webServer := web.NewServer(service)
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.addr, err)
	}
	httpServer := &http.Server{Handler: webServer.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(listener) }()
	logger.Printf("服务监听 http://%s", listener.Addr())
	if cfg.selfcheck {
		err := runSelfcheck(listener.Addr().String())
		ctx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
		defer cancel()
		shutdownErr := httpServer.Shutdown(ctx)
		serverErr := <-serveErr
		if serverErr != nil && !errors.Is(serverErr, http.ErrServerClosed) {
			return serverErr
		}
		if err != nil {
			return err
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		logger.Print("全流程自检通过")
		return nil
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case sig := <-signals:
		logger.Printf("收到 %s，开始优雅关闭", sig)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		return err
	}
	err = <-serveErr
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
