package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"alzette/internal/agentauth"
)

func TestDesktopConnectionInstructionsAreExplicitlySessionScoped(t *testing.T) {
	var output bytes.Buffer
	printDesktopConnection(&output, "http://127.0.0.1:32123/v1", "alp_session_only")
	text := output.String()
	for _, expected := range []string{
		"Desktop connection (valid only while this command is running)",
		"Jan / OpenAI base URL: http://127.0.0.1:32123/v1",
		"Goose API URL: http://127.0.0.1:32123",
		"Goose API base path: v1/chat/completions",
		"Session key: alp_session_only",
		"local one-session key",
		"not your Alzette or OAuth credential",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("desktop instructions omitted %q: %q", expected, text)
		}
	}
}

func TestChooseContextUsesOnlyHumanReadableSelection(t *testing.T) {
	contexts := []agentauth.Context{
		{MembershipID: "mem_one", Organisation: "Example", Project: "Research", Environment: "Development", ModelAliases: []string{"chat"}},
		{MembershipID: "mem_two", Organisation: "Example", Project: "Operations", Environment: "Production", ModelAliases: []string{"summarise"}},
	}
	var output bytes.Buffer
	selected, err := chooseContext(contexts, "", strings.NewReader("2\n"), &output)
	if err != nil || selected.MembershipID != "mem_two" {
		t.Fatalf("selected=%#v err=%v", selected, err)
	}
	if strings.Contains(output.String(), "mem_one") || strings.Contains(output.String(), "mem_two") || !strings.Contains(output.String(), "Operations / Production") {
		t.Fatalf("selection output exposed an opaque ID or omitted human labels: %q", output.String())
	}
}

func TestPreparePiChildUsesIsolatedEmployeeProvider(t *testing.T) {
	arguments, cleanup, err := prepareChild([]string{"pi", "--no-session"}, agentauth.Context{ModelAliases: []string{"alzette-chat"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	joined := strings.Join(arguments, " ")
	if !strings.Contains(joined, "--provider alzette-employee") || !strings.Contains(joined, "--model alzette-chat") || !strings.Contains(joined, "--extension") {
		t.Fatalf("Pi arguments=%q", joined)
	}
	extensionIndex := -1
	for index, argument := range arguments {
		if argument == "--extension" {
			extensionIndex = index + 1
			break
		}
	}
	if extensionIndex <= 0 || extensionIndex >= len(arguments) {
		t.Fatal("Pi extension path is absent")
	}
	content, err := os.ReadFile(arguments[extensionIndex])
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, `registerProvider("alzette-employee"`) || !strings.Contains(text, "supportsStore: false") || strings.Contains(text, "alz_u_") {
		t.Fatal("Pi extension is not isolated or embeds a credential")
	}
}

func TestPreparePiChildPreservesExplicitModelChoice(t *testing.T) {
	arguments, cleanup, err := prepareChild([]string{"pi", "--provider", "alzette-employee", "--model", "second"}, agentauth.Context{ModelAliases: []string{"first", "second"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if strings.Count(strings.Join(arguments, " "), "--provider") != 1 || strings.Count(strings.Join(arguments, " "), "--model") != 1 {
		t.Fatalf("explicit Pi selection was duplicated: %v", arguments)
	}
}
