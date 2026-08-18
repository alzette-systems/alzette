package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"alzette/internal/agentauth"
	"alzette/internal/agentclient"
)

//go:embed pi-extension.ts
var piExtension []byte

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "alzette-agent: %v\n", err)
		os.Exit(1)
	}
}

func run(arguments []string, input io.Reader, output, errorOutput io.Writer) error {
	if len(arguments) == 0 {
		printUsage(errorOutput)
		return errors.New("a command is required")
	}
	command, rest := arguments[0], arguments[1:]
	switch command {
	case "login":
		return login(rest, output)
	case "run":
		return runAgent(rest, input, output, errorOutput, nil)
	case "pi":
		return runAgent(rest, input, output, errorOutput, []string{"pi"})
	case "help", "-h", "--help":
		printUsage(output)
		return nil
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

type options struct {
	controlURL, redirectURL, contextID string
	allowInsecure, noBrowser           bool
}

func flagsFor(name string, output io.Writer) (*flag.FlagSet, *options) {
	values := &options{}
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&values.controlURL, "control", envOr("ALZETTE_CONTROL_URL", "http://127.0.0.1:8081"), "Alzette control origin")
	flags.StringVar(&values.redirectURL, "redirect", envOr("ALZETTE_AGENT_REDIRECT_URL", "http://127.0.0.1:43127/callback"), "registered loopback OAuth redirect")
	flags.StringVar(&values.contextID, "context", "", "advanced: exact Alzette context ID")
	flags.BoolVar(&values.allowInsecure, "allow-insecure-local", envBool("ALZETTE_AGENT_ALLOW_INSECURE_LOCAL"), "allow HTTP for an explicitly local development deployment")
	flags.BoolVar(&values.noBrowser, "no-browser", false, "print the authorization URL instead of opening it")
	return flags, values
}

func login(arguments []string, output io.Writer) error {
	flags, values := flagsFor("login", output)
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("login accepts no positional arguments")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	session, err := agentclient.Login(ctx, clientConfig(*values, output))
	if err != nil {
		return err
	}
	contexts := session.Contexts()
	if len(contexts) == 0 {
		return errors.New("signed in, but no model access is assigned to this employee")
	}
	fmt.Fprintln(output, "Signed in to Alzette.")
	for _, available := range contexts {
		fmt.Fprintf(output, "  %s · %s / %s · %s\n", available.Organisation, available.Project, available.Environment, strings.Join(available.ModelAliases, ", "))
	}
	fmt.Fprintln(output, "No credential was stored. Run `alzette-agent pi` to sign in and start Pi.")
	return nil
}

func runAgent(arguments []string, input io.Reader, output, errorOutput io.Writer, impliedCommand []string) error {
	flags, values := flagsFor("run", errorOutput)
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	childArguments := flags.Args()
	if len(impliedCommand) != 0 {
		childArguments = append(append([]string{}, impliedCommand...), childArguments...)
	}
	if len(childArguments) == 0 {
		return errors.New("usage: alzette-agent run [options] -- <agent> [arguments]")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	session, err := agentclient.Login(ctx, clientConfig(*values, output))
	if err != nil {
		return err
	}
	selected, err := chooseContext(session.Contexts(), values.contextID, input, output)
	if err != nil {
		return err
	}
	if _, err := session.SelectContext(selected.MembershipID); err != nil {
		return err
	}
	if _, _, err := session.EnsureHumanCredential(ctx); err != nil {
		return err
	}
	proxy, err := agentclient.StartProxy(session)
	if err != nil {
		return err
	}
	defer func() {
		shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = proxy.Close(shutdown)
		_ = session.Revoke(shutdown)
	}()
	fmt.Fprintln(output, "Signed in to Alzette.")
	fmt.Fprintf(output, "Company: %s\n", selected.Organisation)
	fmt.Fprintf(output, "Context: %s / %s\n", selected.Project, selected.Environment)
	fmt.Fprintf(output, "Models available: %s\n", strings.Join(selected.ModelAliases, ", "))
	if filepath.Base(childArguments[0]) != "pi" {
		printDesktopConnection(output, proxy.BaseURL(), proxy.Key())
	}

	childArguments, cleanup, err := prepareChild(childArguments, selected, proxy)
	if err != nil {
		return err
	}
	defer cleanup()
	fmt.Fprintf(output, "Starting %s…\n", filepath.Base(childArguments[0]))
	child := exec.CommandContext(ctx, childArguments[0], childArguments[1:]...)
	child.Stdin, child.Stdout, child.Stderr = input, output, errorOutput
	child.Env = append(os.Environ(),
		"OPENAI_BASE_URL="+proxy.BaseURL(),
		"OPENAI_API_KEY="+proxy.Key(),
		"ALZETTE_PI_PROXY_URL="+proxy.BaseURL(),
		"ALZETTE_PI_PROXY_KEY="+proxy.Key(),
	)
	models, _ := json.Marshal(selected.ModelAliases)
	child.Env = append(child.Env, "ALZETTE_PI_MODELS="+string(models))
	if err := child.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return fmt.Errorf("%s exited with status %d", filepath.Base(childArguments[0]), exitError.ExitCode())
		}
		return err
	}
	return nil
}

func printDesktopConnection(output io.Writer, baseURL, key string) {
	fmt.Fprintln(output, "Desktop connection (valid only while this command is running):")
	fmt.Fprintf(output, "  Jan / OpenAI base URL: %s\n", baseURL)
	fmt.Fprintf(output, "  Goose API URL: %s\n", strings.TrimSuffix(baseURL, "/v1"))
	fmt.Fprintln(output, "  Goose API base path: v1/chat/completions")
	fmt.Fprintf(output, "  Session key: %s\n", key)
	fmt.Fprintln(output, "Paste these into the desktop app's OpenAI-compatible provider. This is a local one-session key, not your Alzette or OAuth credential.")
}

func clientConfig(values options, output io.Writer) agentclient.Config {
	openBrowser := agentclient.OpenBrowser
	if values.noBrowser {
		openBrowser = func(string) error { return errors.New("browser disabled") }
	}
	return agentclient.Config{
		ControlURL: values.controlURL, RedirectURL: values.redirectURL,
		AllowInsecure: values.allowInsecure, OpenBrowser: openBrowser, Output: output,
	}
}

func chooseContext(contexts []agentauth.Context, requested string, input io.Reader, output io.Writer) (agentauth.Context, error) {
	if len(contexts) == 0 {
		return agentauth.Context{}, errors.New("no model access is assigned to this employee")
	}
	if requested != "" {
		for _, candidate := range contexts {
			if candidate.MembershipID == requested {
				return candidate, nil
			}
		}
		return agentauth.Context{}, errors.New("the requested Alzette context is unavailable")
	}
	if len(contexts) == 1 {
		return contexts[0], nil
	}
	fmt.Fprintln(output, "Choose where you want to work:")
	for index, candidate := range contexts {
		fmt.Fprintf(output, "  %d. %s · %s / %s · %s\n", index+1, candidate.Organisation, candidate.Project, candidate.Environment, strings.Join(candidate.ModelAliases, ", "))
	}
	fmt.Fprint(output, "Selection: ")
	line, err := bufio.NewReader(io.LimitReader(input, 32)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return agentauth.Context{}, err
	}
	selection, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || selection < 1 || selection > len(contexts) {
		return agentauth.Context{}, errors.New("context selection is invalid")
	}
	return contexts[selection-1], nil
}

func prepareChild(arguments []string, selected agentauth.Context, proxy *agentclient.Proxy) ([]string, func(), error) {
	if filepath.Base(arguments[0]) != "pi" {
		return arguments, func() {}, nil
	}
	temporary, err := os.MkdirTemp("", "alzette-pi-")
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(temporary) }
	extensionPath := filepath.Join(temporary, "alzette.ts")
	if err := os.WriteFile(extensionPath, piExtension, 0600); err != nil {
		cleanup()
		return nil, func() {}, err
	}
	result := append([]string{}, arguments...)
	result = append(result, "--extension", extensionPath)
	if !hasFlag(arguments[1:], "--provider") {
		result = append(result, "--provider", "alzette-employee")
	}
	if !hasFlag(arguments[1:], "--model") && len(selected.ModelAliases) != 0 {
		result = append(result, "--model", selected.ModelAliases[0])
	}
	return result, cleanup, nil
}

func hasFlag(arguments []string, name string) bool {
	for _, argument := range arguments {
		if argument == name || strings.HasPrefix(argument, name+"=") {
			return true
		}
	}
	return false
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string) bool {
	value, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv(name)))
	return value
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, `Usage:
  alzette-agent login [options]
  alzette-agent pi [options] [pi arguments]
  alzette-agent run [options] -- <agent> [arguments]

The employee signs in through their browser. OAuth and Alzette inference
credentials remain inside this process; the child receives only a random
loopback-session capability. No permanent API key is displayed or stored.`)
}
