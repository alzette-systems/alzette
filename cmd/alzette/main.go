package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"alzette/internal/agentauth"
	"alzette/internal/api"
	"alzette/internal/billing"
	stripeadapter "alzette/internal/billing/stripe"
	"alzette/internal/casdoorbootstrap"
	"alzette/internal/catalogue"
	"alzette/internal/control"
	"alzette/internal/endpoints"
	"alzette/internal/faketarget"
	"alzette/internal/federation"
	"alzette/internal/gateway"
	"alzette/internal/humanauth"
	"alzette/internal/platform"
	"alzette/internal/portal"
	"alzette/internal/secrets"
	webserver "alzette/internal/server"
	"alzette/internal/slice0smoke"
	pgstore "alzette/internal/store/postgres"
	"alzette/internal/worker"
	"alzette/internal/workforce"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("alzette: %v", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	command := "serve"
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		command = arguments[0]
		arguments = arguments[1:]
	}
	switch command {
	case "gateway", "control", "serve":
		return runHTTP(command, arguments)
	case "public":
		return runPublic(arguments)
	case "migrate":
		return runMigrate(arguments)
	case "provision":
		return runProvision(arguments)
	case "key":
		return runKey(arguments)
	case "user":
		return runUser(arguments)
	case "ownership":
		return runOwnership(arguments)
	case "identity":
		return runIdentity(arguments)
	case "worker":
		return runWorker(arguments)
	case "catalogue":
		return runCatalogue(arguments)
	case "endpoint":
		return runEndpoint(arguments)
	case "billing-webhook":
		return runBillingWebhook(arguments)
	case "worker-health":
		return runWorkerHealth(arguments)
	case "healthcheck":
		return runHealthcheck(arguments)
	case "fake-target":
		return runFakeTarget(arguments)
	case "slice0-smoke":
		return runSlice0Smoke(arguments)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func runIdentity(arguments []string) error {
	if len(arguments) != 1 || arguments[0] != "bootstrap-casdoor" {
		return errors.New("usage: alzette identity bootstrap-casdoor")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := casdoorbootstrap.Run(ctx, casdoorbootstrap.Config{
		Endpoint: os.Getenv("ALZETTE_CASDOOR_BOOTSTRAP_ENDPOINT"), AdminPassword: os.Getenv("ALZETTE_CASDOOR_ADMIN_PASSWORD"),
		ClientID: os.Getenv("ALZETTE_WORKFORCE_OIDC_CLIENT_ID"), ClientSecret: os.Getenv("ALZETTE_WORKFORCE_OIDC_CLIENT_SECRET"), RedirectURL: os.Getenv("ALZETTE_WORKFORCE_OIDC_REDIRECT_URL"),
		AgentRedirectURL: os.Getenv("ALZETTE_WORKFORCE_AGENT_OIDC_REDIRECT_URL"),
		DemoUsername:     os.Getenv("ALZETTE_CASDOOR_DEMO_USERNAME"), DemoPassword: os.Getenv("ALZETTE_CASDOOR_DEMO_PASSWORD"), DemoEmail: os.Getenv("ALZETTE_CASDOOR_DEMO_EMAIL"), AllowInsecure: envBool("ALZETTE_ALLOW_INSECURE_WORKFORCE_OIDC"),
	})
	if err != nil {
		return err
	}
	return writeOperatorJSON(result)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  alzette gateway [--addr :8080]
  alzette control [--addr :8081] [--static-dir /app/portal]
  alzette public [--addr :8082] [--static-dir /app/public]
  alzette serve [--addr :8080] [--static-dir .]
  alzette migrate
  alzette provision [operator flags]
  alzette key rotate [scope flags]
  alzette key revoke --prefix alz_k_...
  alzette user provision [membership flags]
  alzette user rotate-password --username NAME
  alzette user disable --username NAME
  alzette ownership assign --organisation-slug SLUG --username NAME --evidence-ref REF
  alzette catalogue seed [catalogue flags]
  alzette endpoint quote --request-id ID [commercial flags]
  alzette endpoint transition --request-id ID --state STATE [fulfilment flags]
  alzette billing-webhook [--addr :8083]
  alzette worker [--rollup-interval 1m]
  alzette worker-health [--maximum-age 5m]
  alzette healthcheck [--url http://127.0.0.1:8080/readyz]
  alzette fake-target [--addr :8090]
  alzette slice0-smoke [offline acceptance flags]

DATABASE_URL is required for data-plane, control-plane, migration, provisioning,
key, and Slice 0 smoke commands. Provider credentials are read only through the
configured secret reference; REF_FILE is preferred over REF.`)
}

func runPublic(arguments []string) error {
	flags := flag.NewFlagSet("public", flag.ContinueOnError)
	address := flags.String("addr", envOr("ALZETTE_PUBLIC_ADDR", ":8082"), "HTTP listen address")
	staticDirectory := flags.String("static-dir", envOr("ALZETTE_PUBLIC_STATIC_DIR", "/app/public"), "public site directory")
	portalURL := flags.String("portal-url", envOr("ALZETTE_PUBLIC_PORTAL_URL", "http://127.0.0.1:8081/login"), "customer-visible portal login URL")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	handler, err := newPublicHandler(*staticDirectory, *portalURL)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              *address,
		Handler:           api.SecurityHeaders(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverError := make(chan error, 1)
	go func() {
		log.Printf("Alzette public site listening on %s", *address)
		serverError <- server.ListenAndServe()
	}()
	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownContext.Done():
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func newPublicHandler(staticDirectory, portalTarget string) (http.Handler, error) {
	validatedPortalURL, err := validatePublicPortalURL(portalTarget)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/healthz", webserver.HealthHandler())
	mux.Handle("/readyz", webserver.HealthHandler())
	mux.Handle("/client", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			api.MethodNotAllowed(w, "GET, HEAD", "")
			return
		}
		http.Redirect(w, r, validatedPortalURL, http.StatusSeeOther)
	}))
	mux.Handle("/", webserver.PublicSiteHandler(staticDirectory))
	return mux, nil
}

func validatePublicPortalURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("ALZETTE_PUBLIC_PORTAL_URL must be an absolute HTTP(S) URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", errors.New("ALZETTE_PUBLIC_PORTAL_URL must not contain credentials, a query, or a fragment")
	}
	if parsed.Path == "" {
		parsed.Path = "/login"
	}
	return parsed.String(), nil
}

func runHealthcheck(arguments []string) error {
	flags := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	endpoint := flags.String("url", "http://127.0.0.1:8080/readyz", "local readiness URL")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects are disabled") }}
	response, err := client.Get(*endpoint)
	if err != nil {
		return errors.New("readiness request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("readiness returned status %d", response.StatusCode)
	}
	return nil
}

func runFakeTarget(arguments []string) error {
	flags := flag.NewFlagSet("fake-target", flag.ContinueOnError)
	address := flags.String("addr", ":8090", "HTTP listen address")
	secretRef := flags.String("secret-ref", "SLICE0_FAKE_TARGET_KEY", "fake target secret reference (REF_FILE preferred over REF)")
	providerModel := flags.String("provider-model", faketarget.DefaultProviderModel, "deterministic compatible provider model")
	timeoutFirst := flags.Int64("timeout-first", 1, "number of initial calls that wait for caller timeout")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	secret, ok := secrets.Lookup(*secretRef)
	if !ok || secret == "" {
		return errors.New("fake target credential is unavailable")
	}
	handler, err := faketarget.New(faketarget.Config{Secret: secret, ProviderModel: *providerModel, TimeoutFirst: *timeoutFirst})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              *address,
		Handler:           api.SecurityHeaders(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverError := make(chan error, 1)
	go func() {
		log.Printf("Alzette deterministic Slice 0 target listening on %s", *address)
		serverError <- server.ListenAndServe()
	}()
	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownContext.Done():
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func runSlice0Smoke(arguments []string) error {
	flags := flag.NewFlagSet("slice0-smoke", flag.ContinueOnError)
	gatewayURL := flags.String("gateway-url", "http://gateway:8080", "local gateway URL")
	targetBaseURL := flags.String("target-base-url", "http://fake-target:8090/v1", "local deterministic target compatible base URL")
	providerModel := flags.String("provider-model", faketarget.DefaultProviderModel, "deterministic compatible provider model")
	secretRef := flags.String("secret-ref", "SLICE0_FAKE_TARGET_KEY", "server-side fake target secret reference")
	expectedOutput := flags.String("expected-output", faketarget.ExpectedOutput, "deterministic compatible output marker")
	targetTimeout := flags.Duration("target-timeout", 200*time.Millisecond, "per-attempt deterministic timeout")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if err := slice0smoke.ValidateOfflineEndpoints(*gatewayURL, *targetBaseURL); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database, err := pgstore.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer database.Close()
	if err := pgstore.Migrate(ctx, database); err != nil {
		return err
	}
	store := pgstore.New(database, true)
	result, err := slice0smoke.Run(ctx, slice0smoke.Config{
		Store:          store,
		Provisioner:    store,
		GatewayURL:     *gatewayURL,
		TargetBaseURL:  *targetBaseURL,
		ProviderModel:  *providerModel,
		SecretRef:      *secretRef,
		ExpectedOutput: *expectedOutput,
		TargetTimeout:  *targetTimeout,
	})
	if err != nil {
		return err
	}
	return writeOperatorJSON(result)
}

func runHTTP(mode string, arguments []string) error {
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	defaultAddress := envOr("ALZETTE_ADDR", ":8080")
	if mode == "control" {
		defaultAddress = envOr("ALZETTE_ADDR", ":8081")
	}
	address := flags.String("addr", defaultAddress, "HTTP listen address")
	staticDirectory := flags.String("static-dir", envOr("ALZETTE_STATIC_DIR", "."), "static site directory")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	database, store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer database.Close()
	handler, err := newApplicationHandler(mode, *staticDirectory, database, store)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: *address, Handler: api.SecurityHeaders(handler), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 6 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverError := make(chan error, 1)
	go func() {
		log.Printf("Alzette %s listening on %s", mode, *address)
		serverError <- server.ListenAndServe()
	}()
	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownContext.Done():
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func newApplicationHandler(mode, staticDirectory string, database *sql.DB, store platform.Store) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.Handle("/healthz", webserver.HealthHandler())
	mux.Handle("/api/healthz", webserver.HealthHandler())
	mux.Handle("/readyz", webserver.ReadinessHandler(database))
	if mode == "gateway" || mode == "serve" {
		handler, err := gateway.New(gateway.Config{Store: store, AllowInsecureTargets: envBool("ALZETTE_ALLOW_INSECURE_TARGETS"), AllowedSecretRefs: providerSecretRefs()})
		if err != nil {
			return nil, err
		}
		mux.Handle("/v1/chat/completions", handler)
		mux.Handle("/v1/responses", handler)
		mux.Handle("/v1/messages", handler)
	}
	if mode == "control" || mode == "serve" {
		handler, err := control.New(control.Config{Store: store})
		if err != nil {
			return nil, err
		}
		mux.Handle("/api/v1/dashboard", handler)
		mux.Handle("/api/v1/usage", handler)
		mux.Handle("/api/v1/requests/", handler)
		portalStore, ok := store.(platform.PortalStore)
		if !ok {
			return nil, errors.New("control store does not implement portal sessions")
		}
		sessionTTL, err := envDurationStrict("ALZETTE_PORTAL_SESSION_TTL", 12*time.Hour)
		if err != nil {
			return nil, err
		}
		catalogueService, endpointService, billingService, err := endpointControlServices(store)
		if err != nil {
			return nil, err
		}
		var workforceService *workforce.Service
		if workforceStore, ok := store.(workforce.Store); ok {
			workforceService = workforce.New(workforceStore)
		}
		oidcProvider, err := configuredWorkforceOIDC()
		if err != nil {
			return nil, err
		}
		if oidcProvider != nil {
			accessValidator, validatorOK := oidcProvider.(federation.AccessTokenProvider)
			agentStore, storeOK := store.(agentauth.Store)
			if !validatorOK || !storeOK {
				return nil, errors.New("workforce agent-access capability set is incomplete")
			}
			agentHandler := agentauth.NewHandler(agentauth.New(agentStore, accessValidator), envOr("ALZETTE_PUBLIC_CONTROL_URL", "http://127.0.0.1:8081"), os.Getenv("ALZETTE_PUBLIC_GATEWAY_URL"), os.Getenv("ALZETTE_WORKFORCE_AGENT_OIDC_REDIRECT_URL"))
			mux.Handle("/.well-known/alzette-agent-configuration", agentHandler)
			mux.Handle("/api/agent/", agentHandler)
		}
		site, err := portal.New(portal.Config{
			Store: store, PortalStore: portalStore, StaticDirectory: staticDirectory,
			CookieSecure: envBoolDefault("ALZETTE_PORTAL_COOKIE_SECURE", true), SessionTTL: sessionTTL,
			PublicGatewayURL: os.Getenv("ALZETTE_PUBLIC_GATEWAY_URL"), AllowInsecurePublicGateway: envBool("ALZETTE_ALLOW_INSECURE_PUBLIC_GATEWAY"),
			Catalogue: catalogueService, Endpoints: endpointService, Billing: billingService, Workforce: workforceService, OIDC: oidcProvider,
		})
		if err != nil {
			return nil, err
		}
		mux.Handle("/", site)
	} else {
		mux.Handle("/", webserver.NotFoundHandler())
	}
	return mux, nil
}

func configuredWorkforceOIDC() (federation.Provider, error) {
	config := federation.Config{
		Issuer: os.Getenv("ALZETTE_WORKFORCE_OIDC_ISSUER"), ClientID: os.Getenv("ALZETTE_WORKFORCE_OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("ALZETTE_WORKFORCE_OIDC_CLIENT_SECRET"), RedirectURL: os.Getenv("ALZETTE_WORKFORCE_OIDC_REDIRECT_URL"),
		AllowInsecure: envBool("ALZETTE_ALLOW_INSECURE_WORKFORCE_OIDC"),
	}
	if config.Issuer == "" && config.ClientID == "" && config.ClientSecret == "" && config.RedirectURL == "" {
		return nil, nil
	}
	if config.Issuer == "" || config.ClientID == "" || config.ClientSecret == "" || config.RedirectURL == "" {
		return nil, errors.New("workforce OIDC configuration is incomplete")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return federation.New(ctx, config)
}

func endpointControlServices(store platform.Store) (*catalogue.Service, *endpoints.Service, *billing.Service, error) {
	catalogueStore, hasCatalogue := store.(catalogue.Store)
	endpointStore, hasEndpoints := store.(endpoints.Store)
	billingStore, hasBilling := store.(billing.Store)
	if !hasCatalogue && !hasEndpoints && !hasBilling {
		// The in-memory gateway/control test store predates the optional endpoint
		// marketplace. Keep the established portal and gateway surfaces available;
		// marketplace routes remain fail-closed (404) when the capability set is
		// absent. A production PostgreSQL store implements the complete set below.
		return nil, nil, nil, nil
	}
	if !hasCatalogue || !hasEndpoints || !hasBilling {
		return nil, nil, nil, errors.New("control store endpoint marketplace capability set is incomplete")
	}
	provider, err := configuredBillingProvider()
	if err != nil {
		return nil, nil, nil, err
	}
	billingService, err := billing.New(billing.Config{
		Store: billingStore, Provider: provider,
		SuccessURL: os.Getenv("ALZETTE_BILLING_SUCCESS_URL"),
		CancelURL:  os.Getenv("ALZETTE_BILLING_CANCEL_URL"),
		ReturnURL:  os.Getenv("ALZETTE_BILLING_RETURN_URL"),
	})
	if err != nil {
		return nil, nil, nil, err
	}
	catalogueService, err := catalogue.New(catalogueStore, func() (bool, string) {
		capability := provider.Capability()
		return capability.Configured, capability.Provider
	})
	if err != nil {
		return nil, nil, nil, err
	}
	endpointService, err := endpoints.New(endpointStore, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	return catalogueService, endpointService, billingService, nil
}

func configuredBillingProvider() (billing.Provider, error) {
	mode := strings.TrimSpace(os.Getenv("ALZETTE_BILLING_PROVIDER"))
	if mode == "" || mode == "disabled" {
		return billing.UnavailableProvider{}, nil
	}
	if mode != "stripe" {
		return nil, errors.New("ALZETTE_BILLING_PROVIDER must be disabled or stripe")
	}
	key, ok := secrets.Lookup("STRIPE_SECRET_KEY")
	if !ok || key == "" {
		return nil, errors.New("Stripe billing is enabled but STRIPE_SECRET_KEY_FILE is unavailable")
	}
	return stripeadapter.NewProvider(stripeadapter.Config{APIKey: key})
}

func openStore(ctx context.Context) (*sql.DB, *pgstore.Store, error) {
	database, err := pgstore.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, nil, err
	}
	return database, pgstore.New(database, envBool("ALZETTE_ALLOW_INSECURE_TARGETS")), nil
}

func runMigrate(arguments []string) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("migrate accepts no arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database, err := pgstore.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer database.Close()
	if err := pgstore.Migrate(ctx, database); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "migrations applied")
	return nil
}

func runProvision(arguments []string) error {
	flags := flag.NewFlagSet("provision", flag.ContinueOnError)
	spec := platform.ProvisionSpec{}
	flags.StringVar(&spec.OrganisationName, "organisation-name", "", "organisation display name")
	flags.StringVar(&spec.OrganisationSlug, "organisation-slug", "", "organisation slug")
	flags.StringVar(&spec.ProjectName, "project-name", "", "project display name")
	flags.StringVar(&spec.ProjectSlug, "project-slug", "", "project slug")
	flags.StringVar(&spec.EnvironmentName, "environment-name", "Production", "environment display name")
	flags.StringVar(&spec.EnvironmentSlug, "environment-slug", "production", "environment slug")
	flags.StringVar(&spec.ModelAlias, "model-alias", "", "customer-facing model alias")
	flags.StringVar(&spec.ModelVersion, "model-version", "poc", "approved model version")
	flags.StringVar(&spec.TargetName, "target-name", "openrouter-pilot", "operator target name")
	flags.StringVar(&spec.ExecutionClass, "execution-class", "external_pilot", "external_pilot or private_compatible")
	flags.StringVar(&spec.CapacityMode, "capacity-mode", "shared", "shared or dedicated")
	flags.StringVar(&spec.CapacityEvidenceRef, "capacity-evidence-ref", "", "operator evidence reference required for dedicated capacity")
	flags.StringVar(&spec.TargetBaseURL, "target-base-url", envOr("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"), "operator-controlled compatible base URL")
	flags.StringVar(&spec.ProviderModel, "provider-model", os.Getenv("OPENROUTER_MODEL"), "operator-controlled provider model slug")
	flags.StringVar(&spec.SecretRef, "secret-ref", "OPENROUTER_API_KEY", "server-side provider secret reference (REF_FILE preferred over REF)")
	flags.DurationVar(&spec.TargetTimeout, "target-timeout", 30*time.Second, "per-attempt target timeout")
	flags.IntVar(&spec.MaxAttempts, "max-attempts", 2, "maximum pre-output attempts")
	flags.BoolVar(&spec.ProbeEnabled, "probe-enabled", false, "opt in this target to metadata-only compatible readiness probes")
	flags.DurationVar(&spec.ProbeInterval, "probe-interval", 5*time.Minute, "freshness interval for an opted-in target probe")
	flags.StringVar(&spec.ServiceAccount, "service-account", "application", "service account name")
	flags.StringVar(&spec.ServicePlanCode, "service-plan-code", "", "organisation-scoped operator plan code (optional)")
	flags.StringVar(&spec.ServicePlanName, "service-plan-name", "", "operator plan display name")
	sharedRequestAllowance := flags.String("shared-request-allowance", "", "optional non-negative logical-request allowance")
	flags.StringVar(&spec.SharedRequestAllowancePeriod, "shared-request-allowance-period", "", "hour, day, month, or contract_term")
	sharedTokenAllowance := flags.String("shared-token-allowance", "", "optional non-negative provider-reported-token allowance")
	flags.StringVar(&spec.SharedTokenAllowancePeriod, "shared-token-allowance-period", "", "hour, day, month, or contract_term")
	flags.StringVar(&spec.DedicatedResourceClass, "dedicated-resource-class", "", "optional declared dedicated resource class")
	dedicatedAccelerators := flags.String("dedicated-accelerator-count", "", "optional positive dedicated accelerator allocation")
	planSource := flags.String("service-plan-source", "operator_provisioning", "safe operator evidence label")
	planFinality := flags.String("service-plan-finality", "declared", "declared or unknown")
	scopeValue := flags.String("scopes", strings.Join(defaultScopes(), ","), "comma-separated key scopes")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	spec.Scopes = splitScopes(*scopeValue)
	var err error
	if spec.ServicePlanCode != "" {
		spec.ServicePlanSource, spec.ServicePlanFinality = *planSource, *planFinality
	}
	if spec.SharedRequestAllowance, err = optionalInt64(*sharedRequestAllowance); err != nil {
		return fmt.Errorf("shared request allowance: %w", err)
	}
	if spec.SharedTokenAllowance, err = optionalInt64(*sharedTokenAllowance); err != nil {
		return fmt.Errorf("shared token allowance: %w", err)
	}
	if spec.DedicatedAcceleratorCount, err = optionalInt64(*dedicatedAccelerators); err != nil {
		return fmt.Errorf("dedicated accelerator count: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database, store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := pgstore.Migrate(ctx, database); err != nil {
		return err
	}
	result, err := store.Provision(ctx, spec)
	if err != nil {
		return err
	}
	return writeOperatorJSON(result)
}

func runKey(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("key requires rotate or revoke")
	}
	command := arguments[0]
	arguments = arguments[1:]
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer database.Close()
	switch command {
	case "rotate":
		flags := flag.NewFlagSet("key rotate", flag.ContinueOnError)
		spec := platform.RotateKeySpec{}
		flags.StringVar(&spec.OrganisationSlug, "organisation-slug", "", "organisation slug")
		flags.StringVar(&spec.ProjectSlug, "project-slug", "", "project slug")
		flags.StringVar(&spec.EnvironmentSlug, "environment-slug", "production", "environment slug")
		flags.StringVar(&spec.ServiceAccount, "service-account", "application", "service account name")
		scopeValue := flags.String("scopes", strings.Join(defaultScopes(), ","), "comma-separated key scopes")
		if err := flags.Parse(arguments); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected positional arguments")
		}
		spec.Scopes = splitScopes(*scopeValue)
		result, err := store.RotateKey(ctx, spec)
		if err != nil {
			return err
		}
		return writeOperatorJSON(result)
	case "revoke":
		flags := flag.NewFlagSet("key revoke", flag.ContinueOnError)
		prefix := flags.String("prefix", "", "non-secret API key prefix")
		if err := flags.Parse(arguments); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected positional arguments")
		}
		if err := store.RevokeKey(ctx, *prefix); err != nil {
			return err
		}
		return writeOperatorJSON(map[string]interface{}{"key_prefix": *prefix, "revoked": true})
	default:
		return fmt.Errorf("unknown key command %q", command)
	}
}

func runUser(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("user requires provision, rotate-password, or disable")
	}
	command := arguments[0]
	arguments = arguments[1:]
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database, store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := pgstore.Migrate(ctx, database); err != nil {
		return err
	}
	switch command {
	case "provision":
		flags := flag.NewFlagSet("user provision", flag.ContinueOnError)
		spec := platform.HumanUserSpec{}
		flags.StringVar(&spec.Username, "username", "", "portal username")
		flags.StringVar(&spec.DisplayName, "display-name", "", "human display name")
		flags.StringVar(&spec.OrganisationSlug, "organisation-slug", "", "existing organisation slug")
		flags.StringVar(&spec.ProjectSlug, "project-slug", "", "existing project slug")
		flags.StringVar(&spec.EnvironmentSlug, "environment-slug", "production", "existing environment slug")
		flags.StringVar(&spec.Role, "role", platform.PortalRoleProjectAdmin, "org_admin, project_admin, developer, or viewer")
		passwordFile := flags.String("password-file", "", "optional operator-readable password file; omit to generate once")
		if err := flags.Parse(arguments); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected positional arguments")
		}
		password, generated, err := humanPassword(*passwordFile)
		if err != nil {
			return err
		}
		spec.PasswordHash, err = humanauth.HashPassword(password)
		if err != nil {
			return err
		}
		result, err := store.ProvisionHuman(ctx, spec)
		if err != nil {
			return err
		}
		output := map[string]interface{}{"user_id": result.UserID, "username": result.Username, "membership_id": result.MembershipID, "created": result.Created}
		if generated && result.Created {
			output["one_time_password"] = password
		}
		return writeOperatorJSON(output)
	case "rotate-password":
		flags := flag.NewFlagSet("user rotate-password", flag.ContinueOnError)
		username := flags.String("username", "", "portal username")
		passwordFile := flags.String("password-file", "", "optional operator-readable password file; omit to generate once")
		if err := flags.Parse(arguments); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected positional arguments")
		}
		password, generated, err := humanPassword(*passwordFile)
		if err != nil {
			return err
		}
		hash, err := humanauth.HashPassword(password)
		if err != nil {
			return err
		}
		if err := store.RotateHumanPassword(ctx, *username, hash); err != nil {
			return err
		}
		output := map[string]interface{}{"username": *username, "password_rotated": true, "sessions_revoked": true}
		if generated {
			output["one_time_password"] = password
		}
		return writeOperatorJSON(output)
	case "disable":
		flags := flag.NewFlagSet("user disable", flag.ContinueOnError)
		username := flags.String("username", "", "portal username")
		if err := flags.Parse(arguments); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected positional arguments")
		}
		if err := store.DisableHuman(ctx, *username); err != nil {
			return err
		}
		return writeOperatorJSON(map[string]interface{}{"username": *username, "disabled": true, "sessions_revoked": true})
	default:
		return fmt.Errorf("unknown user command %q", command)
	}
}

func runOwnership(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("ownership requires assign")
	}
	command := arguments[0]
	arguments = arguments[1:]
	if command != "assign" {
		return fmt.Errorf("unknown ownership command %q", command)
	}
	flags := flag.NewFlagSet("ownership assign", flag.ContinueOnError)
	spec := workforce.InitialOwnerSpec{}
	flags.StringVar(&spec.OrganisationSlug, "organisation-slug", "", "existing organisation slug")
	flags.StringVar(&spec.Username, "username", "", "enabled portal username in the organisation")
	flags.StringVar(&spec.EvidenceRef, "evidence-ref", "", "operator evidence reference for the initial owner decision")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := pgstore.Migrate(ctx, database); err != nil {
		return err
	}
	result, err := workforce.New(store).AssignInitialOwner(ctx, spec)
	if err != nil {
		return err
	}
	return writeOperatorJSON(result)
}

func humanPassword(filename string) (string, bool, error) {
	if filename == "" {
		value, err := humanauth.GeneratePassword()
		return value, true, err
	}
	file, err := os.Open(filename)
	if err != nil {
		return "", false, errors.New("password file is unavailable")
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, 129))
	if err != nil || len(contents) > 128 {
		return "", false, errors.New("password file is invalid")
	}
	value := strings.TrimRight(string(contents), "\r\n")
	if err := humanauth.ValidatePassword(value); err != nil {
		return "", false, err
	}
	return value, false, nil
}

func runCatalogue(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "seed" {
		return errors.New("catalogue requires seed")
	}
	flags := flag.NewFlagSet("catalogue seed", flag.ContinueOnError)
	spec := catalogue.SeedSpec{}
	flags.StringVar(&spec.ModelAlias, "model-alias", "", "existing server-owned model alias")
	flags.StringVar(&spec.TargetName, "target-name", "", "existing shared target operator name")
	flags.StringVar(&spec.ModelSlug, "model-slug", "", "customer-safe catalogue slug")
	flags.StringVar(&spec.ModelName, "model-name", "", "catalogue display name")
	flags.StringVar(&spec.ModelFamily, "model-family", "", "catalogue family")
	flags.StringVar(&spec.Description, "description", "", "customer-safe description")
	flags.StringVar(&spec.ReleaseVersion, "release-version", "", "exact approved release matching the model registry")
	contextWindow := flags.Int64("context-window-tokens", 0, "reviewed context window; zero leaves unavailable")
	flags.StringVar(&spec.LicenceName, "licence-name", "operator-reviewed", "reviewed licence name")
	flags.StringVar(&spec.LicenceStatus, "licence-status", "approved", "approved or restricted")
	flags.StringVar(&spec.SupportStatus, "support-status", "supported", "supported or limited")
	flags.StringVar(&spec.SourceLabel, "source", "operator_catalogue", "safe evidence source label")
	flags.StringVar(&spec.EvidenceRef, "evidence-ref", "operator-catalogue-v1", "safe evidence reference")
	flags.StringVar(&spec.SharedProfileCode, "shared-profile-code", "shared-evaluation", "shared profile code")
	flags.StringVar(&spec.SharedProfileName, "shared-profile-name", "Shared evaluation", "shared profile name")
	flags.StringVar(&spec.RuntimeClass, "runtime-class", "compatible-chat", "customer-safe runtime class")
	flags.StringVar(&spec.EvaluationOfferCode, "evaluation-offer-code", "shared-evaluation", "free evaluation offer code")
	flags.StringVar(&spec.EvaluationOfferName, "evaluation-offer-name", "Shared evaluation", "free evaluation offer name")
	flags.Int64Var(&spec.EvaluationRequestLimit, "evaluation-request-limit", 100, "hard logical-request evaluation allowance")
	evaluationTokens := flags.Int64("evaluation-token-limit", 0, "optional hard provider-reported token allowance")
	flags.BoolVar(&spec.EligibleEvaluation, "eligible-evaluation", true, "publish for evaluation organisations")
	flags.BoolVar(&spec.EligibleCustomer, "eligible-customer", true, "publish for customer organisations")
	flags.StringVar(&spec.PaidOfferCode, "paid-offer-code", "", "optional paid shared offer code")
	flags.StringVar(&spec.PaidOfferName, "paid-offer-name", "", "paid shared offer name")
	flags.StringVar(&spec.PaidCurrency, "paid-currency", "EUR", "fixed recurring price currency")
	flags.Int64Var(&spec.PaidAmountMinor, "paid-amount-minor", 0, "fixed monthly recurring amount in minor units")
	flags.Int64Var(&spec.PaidRequestLimit, "paid-request-limit", 0, "hard monthly logical-request allowance")
	flags.StringVar(&spec.StripePriceRef, "stripe-price-ref", "", "operator-owned Stripe recurring Price reference")
	flags.StringVar(&spec.DedicatedProfileCode, "dedicated-profile-code", "", "optional dedicated profile and offer code")
	flags.StringVar(&spec.DedicatedProfileName, "dedicated-profile-name", "", "dedicated profile display name")
	flags.StringVar(&spec.DedicatedExecutionClass, "dedicated-execution-class", "private_compatible", "private_compatible or meluxina")
	flags.StringVar(&spec.DedicatedRuntimeClass, "dedicated-runtime-class", "", "customer-safe dedicated runtime class")
	flags.StringVar(&spec.AcceleratorClass, "accelerator-class", "", "reviewed accelerator class")
	flags.IntVar(&spec.AcceleratorsPerUnit, "accelerators-per-unit", 0, "accelerators per capacity unit")
	flags.IntVar(&spec.MinimumCapacityUnits, "minimum-capacity-units", 1, "minimum dedicated units")
	flags.IntVar(&spec.MaximumCapacityUnits, "maximum-capacity-units", 1, "maximum dedicated units")
	flags.StringVar(&spec.DedicatedCapacityFinality, "dedicated-capacity-finality", "contractual", "estimated, measured, or contractual")
	flags.StringVar(&spec.DedicatedEvidenceRef, "dedicated-evidence-ref", "", "safe dedicated capacity evidence reference")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *contextWindow > 0 {
		spec.ContextWindowTokens = contextWindow
	}
	if *evaluationTokens > 0 {
		spec.EvaluationTokenLimit = evaluationTokens
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database, store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := pgstore.Migrate(ctx, database); err != nil {
		return err
	}
	result, err := store.SeedCatalogue(ctx, spec)
	if err != nil {
		return err
	}
	return writeOperatorJSON(result)
}

func runEndpoint(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("endpoint requires quote or transition")
	}
	command := arguments[0]
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database, store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := pgstore.Migrate(ctx, database); err != nil {
		return err
	}
	switch command {
	case "quote":
		flags := flag.NewFlagSet("endpoint quote", flag.ContinueOnError)
		spec := endpoints.QuoteSpec{}
		flags.StringVar(&spec.RequestID, "request-id", "", "deployment request ID")
		flags.StringVar(&spec.Currency, "currency", "EUR", "three-letter contractual currency")
		flags.StringVar(&spec.BillingPeriod, "billing-period", "month", "month or contract_term")
		flags.Int64Var(&spec.RecurringUnitAmountMinor, "recurring-unit-amount-minor", 0, "contractual amount per capacity unit")
		flags.Int64Var(&spec.SetupTotalAmountMinor, "setup-total-amount-minor", 0, "contractual setup amount")
		flags.StringVar(&spec.TaxTreatment, "tax-treatment", "not_determined", "not_determined, exclusive, inclusive, or not_applicable")
		flags.StringVar(&spec.PriceFinality, "price-finality", "contractual", "must be contractual for acceptance")
		flags.StringVar(&spec.CollectionMode, "collection-mode", "invoice", "checkout_payment, invoice, invoice_terms, or not_required")
		dueDays := flags.Int("payment-due-days", 0, "required only for invoice_terms")
		expiresIn := flags.Duration("expires-in", 14*24*time.Hour, "quote validity")
		flags.StringVar(&spec.SourceLabel, "source", "operator_quote", "safe quote source")
		flags.StringVar(&spec.EvidenceRef, "evidence-ref", "", "safe quote evidence reference")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || *expiresIn < time.Hour || *expiresIn > 90*24*time.Hour {
			return errors.New("invalid endpoint quote arguments")
		}
		if *dueDays > 0 {
			spec.PaymentDueDays = dueDays
		}
		spec.ExpiresAt = time.Now().UTC().Add(*expiresIn)
		result, err := store.IssueDeploymentQuote(ctx, spec)
		if err != nil {
			return err
		}
		return writeOperatorJSON(result)
	case "transition":
		flags := flag.NewFlagSet("endpoint transition", flag.ContinueOnError)
		spec := endpoints.TransitionSpec{}
		flags.StringVar(&spec.RequestID, "request-id", "", "deployment request ID")
		flags.StringVar(&spec.State, "state", "", "approved, allocating, deploying, validating, ready, or failed")
		flags.StringVar(&spec.EvidenceRef, "evidence-ref", "", "safe validation/allocation evidence reference")
		flags.StringVar(&spec.TargetName, "target-name", "", "existing owned target; required only for ready")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected positional arguments")
		}
		result, err := store.TransitionDeploymentRequest(ctx, spec)
		if err != nil {
			return err
		}
		return writeOperatorJSON(result)
	default:
		return fmt.Errorf("unknown endpoint command %q", command)
	}
}

func runBillingWebhook(arguments []string) error {
	flags := flag.NewFlagSet("billing-webhook", flag.ContinueOnError)
	address := flags.String("addr", envOr("ALZETTE_BILLING_WEBHOOK_ADDR", ":8083"), "HTTP listen address")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	database, store, err := openStore(ctx)
	cancel()
	if err != nil {
		return err
	}
	defer database.Close()
	var verifier billing.Verifier = stripeadapter.DisabledVerifier{}
	if strings.TrimSpace(os.Getenv("ALZETTE_BILLING_PROVIDER")) == "stripe" {
		secret, ok := secrets.Lookup("STRIPE_WEBHOOK_SECRET")
		if !ok || secret == "" {
			return errors.New("Stripe billing is enabled but STRIPE_WEBHOOK_SECRET_FILE is unavailable")
		}
		verifier, err = stripeadapter.NewVerifier(stripeadapter.VerifierConfig{WebhookSecret: secret, AllowLive: envBool("ALZETTE_STRIPE_ALLOW_LIVE_MODE")})
		if err != nil {
			return err
		}
	}
	handler, err := billing.NewWebhookHandler(billing.WebhookConfig{Store: store, Verifier: verifier})
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/healthz", webserver.HealthHandler())
	mux.Handle("/readyz", webserver.ReadinessHandler(database))
	mux.Handle("/webhooks/stripe", handler)
	server := &http.Server{Addr: *address, Handler: api.SecurityHeaders(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverError := make(chan error, 1)
	go func() {
		log.Printf("Alzette billing webhook listener active on %s", *address)
		serverError <- server.ListenAndServe()
	}()
	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownContext.Done():
	}
	shutdown, stopShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopShutdown()
	return server.Shutdown(shutdown)
}

func runWorker(arguments []string) error {
	flags := flag.NewFlagSet("worker", flag.ContinueOnError)
	interval := flags.Duration("rollup-interval", envDuration("ALZETTE_ROLLUP_INTERVAL", time.Minute), "usage reconciliation interval")
	lookback := flags.Duration("rollup-lookback", envDuration("ALZETTE_ROLLUP_LOOKBACK", 48*time.Hour), "usage reconciliation lookback")
	probesEnabled := flags.Bool("enable-probes", envBool("ALZETTE_ENABLE_TARGET_PROBES"), "globally enable targets individually opted in to probes")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	database, store, err := openStore(ctx)
	cancel()
	if err != nil {
		return err
	}
	defer database.Close()
	allowed := make(map[string]bool)
	for _, reference := range providerSecretRefs() {
		allowed[reference] = true
	}
	runner, err := worker.New(worker.Config{Store: store, RollupInterval: *interval, RollupLookback: *lookback, ProbesEnabled: *probesEnabled, AllowInsecureTargets: envBool("ALZETTE_ALLOW_INSECURE_TARGETS"), AllowedSecretRefs: allowed})
	if err != nil {
		return err
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runner.Run(shutdownContext)
}

func runWorkerHealth(arguments []string) error {
	flags := flag.NewFlagSet("worker-health", flag.ContinueOnError)
	maximumAge := flags.Duration("maximum-age", 5*time.Minute, "maximum acceptable successful rollup checkpoint age")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	database, store, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer database.Close()
	return store.WorkerHealthy(ctx, time.Now().UTC(), *maximumAge)
}

func writeOperatorJSON(value interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
func defaultScopes() []string {
	return []string{platform.ScopeInferenceWrite, platform.ScopeRoutesRead, platform.ScopeUsageRead}
}
func splitScopes(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func envBool(name string) bool {
	value, err := strconv.ParseBool(os.Getenv(name))
	return err == nil && value
}

func envBoolDefault(name string, fallback bool) bool {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDurationStrict(name string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid", name)
	}
	return parsed, nil
}

func optionalInt64(value string) (*int64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, platform.ErrInvalid
	}
	return &parsed, nil
}

func providerSecretRefs() []string {
	return splitScopes(envOr("ALZETTE_PROVIDER_SECRET_REFS", "OPENROUTER_API_KEY"))
}
