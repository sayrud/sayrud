// Package cmd implements the commands of sayrud-server.
package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"

	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/route"
	"github.com/wuhan005/sayrud/internal/setup"
)

const (
	defaultPort       = 2830
	defaultConfigPath = "./config/sayrud.yaml"
	// setupWatchInterval is how often the wizard checks whether another instance has finished the installation.
	setupWatchInterval = 3 * time.Second
)

// Web serves the web interface and the API, or the setup wizard until the installation finishes.
var Web = &cli.Command{
	Name:        "web",
	Usage:       "Start the web server",
	Description: "Serves the web interface and the API, or the setup wizard until the installation finishes.",
	Action:      runWeb,
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:      "port",
			Aliases:   []string{"p"},
			Value:     defaultPort,
			Usage:     "listening port",
			Validator: validatePort,
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   defaultConfigPath,
			Usage:   "path to the configuration file",
		},
	},
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return errors.Newf("port %d is not between 1 and 65535", port)
	}
	return nil
}

func runWeb(ctx context.Context, c *cli.Command) error {
	signals, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	app := &application{}
	defer app.close()

	port := c.Int("port")
	handler := &switchHandler{}
	installer, err := prepare(ctx, c.String("config"), app, handler)
	if err != nil {
		return errors.Wrap(err, "start")
	}

	if signals.Err() != nil {
		return nil
	}

	address := fmt.Sprintf("0.0.0.0:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return errors.Wrapf(err, "listen on %s", address)
	}
	logrus.Infof("Listening on %s", address)

	if installer != nil {
		installer.announce(port)
		go installer.wizard.Watch(signals, setupWatchInterval)
	}

	requests, cancelRequests := context.WithCancel(ctx)
	defer cancelRequests()
	server := &http.Server{
		Handler:           handler,
		BaseContext:       func(net.Listener) context.Context { return requests },
		ReadHeaderTimeout: 10 * time.Second,
	}

	if err := serve(signals, listener, server, app, cancelRequests, conf.App.DrainDelay, conf.App.ShutdownTimeout); err != nil {
		logrus.WithError(err).Error("Server stopped with incomplete graceful shutdown")
	}
	logrus.Info("Shutting down")
	return nil
}

// installer is the installation wizard served until the installation finishes.
type installer struct {
	wizard  *setup.Wizard
	token   *setup.Token
	fromEnv bool
}

// announce logs where to finish the installation, once the server accepts connections.
func (i *installer) announce(port int) {
	setupURL := fmt.Sprintf("http://localhost:%d%s", port, route.SetupPath)
	if i.fromEnv {
		logrus.Warnf("Sayrud is not installed yet, open %s and enter the setup token set by %s", setupURL, setup.TokenEnv)
		return
	}
	logrus.Warnf("Sayrud is not installed yet, open %s?token=%s to finish the installation", setupURL, i.token)
	logrus.Warnf("Setup token: %s", i.token)
}

// prepare starts the application and serves it if installed, otherwise it serves the installation wizard, which is returned.
func prepare(ctx context.Context, configPath string, app *application, handler *switchHandler) (*installer, error) {
	stage := setup.StageDatabase
	if _, err := os.Stat(configPath); err == nil {
		if err := conf.Init(configPath); err != nil {
			return nil, errors.Wrap(err, "initialize configuration")
		}
		if err := app.start(ctx, nil); err != nil {
			return nil, err
		}
		exists, err := db.Users.Exists(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "check users")
		}
		if exists {
			handler.Store(app.Router())
			return nil, nil
		}
		stage = setup.StageAdmin
	} else if errors.Is(err, os.ErrNotExist) {
		if err := conf.InitDefault(); err != nil {
			return nil, errors.Wrap(err, "initialize default configuration")
		}
	} else {
		return nil, errors.Wrap(err, "stat configuration file")
	}

	token, fromEnv, err := setupToken()
	if err != nil {
		return nil, err
	}
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "resolve configuration path")
	}

	wizard := setup.New(setup.Options{
		Token:      token,
		Stage:      stage,
		ConfigPath: absConfigPath,
		Hooks: setup.Hooks{
			// The application outlives the request, so it starts with the process context.
			ConfigureDatabase: func(_ context.Context, opts setup.DatabaseOptions) error {
				return configureDatabase(ctx, configPath, app, opts)
			},
			Finish: func() {
				handler.Store(app.Router())
				logrus.Info("Installation completed, serving the application")
			},
		},
	})
	handler.Store(route.NewSetup(wizard))
	return &installer{wizard: wizard, token: token, fromEnv: fromEnv}, nil
}

// setupToken returns the token set by setup.TokenEnv, or a generated one.
func setupToken() (*setup.Token, bool, error) {
	if value := os.Getenv(setup.TokenEnv); value != "" {
		token, err := setup.ParseToken(value)
		if err != nil {
			return nil, false, errors.Wrapf(err, "invalid %s", setup.TokenEnv)
		}
		return token, true, nil
	}
	token, err := setup.NewToken()
	if err != nil {
		return nil, false, errors.Wrap(err, "generate setup token")
	}
	return token, false, nil
}

// configureDatabase saves the configuration entered in the wizard and starts the application behind the wizard.
// The configuration file only appears after all the services connect, so a failure can be retried.
func configureDatabase(ctx context.Context, configPath string, app *application, opts setup.DatabaseOptions) error {
	secretKey, err := setup.GenerateSecretKey()
	if err != nil {
		return err
	}
	data, err := setup.RenderConfig(setup.ConfigOptions{
		Postgres:  opts.Postgres,
		Redis:     opts.Redis,
		SecretKey: secretKey,
	})
	if err != nil {
		return errors.Wrap(err, "render configuration")
	}

	pending, err := setup.WriteConfig(configPath, data)
	if err != nil {
		return errors.Wrap(err, "write configuration")
	}
	defer pending.Discard()
	if err := conf.Init(pending.TempPath()); err != nil {
		return errors.Wrap(err, "load configuration")
	}

	committed := false
	err = app.start(ctx, func() error {
		if err := pending.Commit(); err != nil {
			return err
		}
		committed = true
		logrus.Infof("Configuration saved to %s", configPath)
		return nil
	})
	if err != nil && committed {
		return errors.Mark(err, setup.ErrRestartRequired)
	}
	return err
}

// serve reserves part of the total budget for canceled workers to release their
// jobs. Ordinary requests keep their context until grace expires.
func serve(signals context.Context, listener net.Listener, server *http.Server, runtime lifecycle, cancelRequests context.CancelFunc, drainDelay, timeout time.Duration) error {
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()

	var serveErr error
	select {
	case <-signals.Done():
	case serveErr = <-served:
	}

	runtime.BeginDrain()

	total, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	grace, cancelGrace := context.WithTimeout(total, conf.ShutdownGracePeriod(timeout))
	defer cancelGrace()

	timer := time.NewTimer(drainDelay)
	select {
	case <-timer.C:
	case <-grace.Done():
	}
	timer.Stop()

	httpErr := server.Shutdown(grace)
	if httpErr != nil {
		cancelRequests()
		_ = server.Close()
	}

	runtimeErr := runtime.Shutdown(grace, total)
	cancelRequests()

	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	if httpErr != nil {
		return httpErr
	}
	return runtimeErr
}
