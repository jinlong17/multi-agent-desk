package main

import (
	"flag"
	"os"
	"strings"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func runRegistryAccounts(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "accounts command is required")
	}
	action := args[0]
	target, flagArgs, err := targetAndFlags(action, args[1:], map[string]bool{"show": true, "update": true, "disable": true, "enable": true, "delete": true})
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("accounts "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	jsonOutput := flags.Bool("json", false, "JSON output")
	provider := flags.String("provider", "", "provider filter or create value")
	name := flags.String("name", "", "display name")
	alias := flags.String("alias", "", "default profile alias")
	subscription := flags.String("subscription-hint", "", "non-authoritative subscription hint")
	limit := flags.Int("limit", 0, "page limit")
	cursor := flags.String("cursor", "", "opaque page cursor")
	revision := flags.Int64("if-revision", 0, "expected entity revision")
	confirm := flags.Bool("confirm", false, "confirm deletion")
	if err := flags.Parse(flagArgs); err != nil || *root == "" || len(flags.Args()) != 0 {
		return domain.NewError(domain.CodeInvalidArgument, "accounts command arguments are invalid")
	}
	switch action {
	case "add":
		if *provider == "" || *name == "" || *alias == "" {
			return domain.NewError(domain.CodeInvalidArgument, "accounts add requires --provider, --name, and --alias")
		}
		body := map[string]any{"provider": *provider, "name": *name, "alias": *alias}
		if *subscription != "" {
			body["subscription_hint"] = *subscription
		}
		return runRPCCommand(*root, "accounts.create", domain.CapabilityClientAdmin, body, nil, true, *jsonOutput, stdout)
	case "list":
		body := map[string]any{"limit": *limit}
		if *provider != "" {
			body["provider"] = *provider
		}
		if *cursor != "" {
			body["cursor"] = *cursor
		}
		return runRPCCommand(*root, "accounts.list", domain.CapabilityMetadataRead, body, nil, false, *jsonOutput, stdout)
	case "show":
		return runRPCCommand(*root, "accounts.show", domain.CapabilityMetadataRead, map[string]any{"target": target}, nil, false, *jsonOutput, stdout)
	case "update":
		if *revision < 1 {
			return domain.NewError(domain.CodeInvalidArgument, "accounts update requires --if-revision")
		}
		body := map[string]any{"target": target, "expected_revision": *revision}
		visited := visitedFlags(flags)
		if visited["name"] {
			body["name"] = *name
		}
		if visited["subscription-hint"] {
			body["subscription_hint"] = *subscription
		}
		if len(body) == 2 {
			return domain.NewError(domain.CodeInvalidArgument, "accounts update requires a changed field")
		}
		return runRPCCommand(*root, "accounts.update", domain.CapabilityClientAdmin, body, nil, true, *jsonOutput, stdout)
	case "disable", "enable":
		if *revision < 1 {
			return domain.NewError(domain.CodeInvalidArgument, "account state change requires --if-revision")
		}
		return runRPCCommand(*root, "accounts."+action, domain.CapabilityClientAdmin,
			map[string]any{"target": target, "expected_revision": *revision}, nil, true, *jsonOutput, stdout)
	case "delete":
		if *revision < 1 || !*confirm {
			return domain.NewError(domain.CodeInvalidArgument, "accounts delete requires --if-revision and --confirm")
		}
		return runRPCCommand(*root, "accounts.delete", domain.CapabilityClientAdmin,
			map[string]any{"target": target, "expected_revision": *revision}, nil, true, *jsonOutput, stdout)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "accounts command is not available")
	}
}

func runRegistryProfiles(args []string, stdout, stderr *os.File) error {
	if len(args) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, "profiles command is required")
	}
	action := args[0]
	target, flagArgs, err := targetAndFlags(action, args[1:], map[string]bool{"show": true, "validate": true, "update": true, "disable": true, "enable": true, "delete": true})
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("profiles "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	jsonOutput := flags.Bool("json", false, "JSON output")
	account := flags.String("account", "", "account ID")
	name := flags.String("name", "", "display name")
	alias := flags.String("alias", "", "profile alias")
	limit := flags.Int("limit", 0, "page limit")
	cursor := flags.String("cursor", "", "opaque page cursor")
	revision := flags.Int64("if-revision", 0, "expected entity revision")
	confirm := flags.Bool("confirm", false, "confirm deletion")
	if err := flags.Parse(flagArgs); err != nil || *root == "" || len(flags.Args()) != 0 {
		return domain.NewError(domain.CodeInvalidArgument, "profiles command arguments are invalid")
	}
	switch action {
	case "create":
		if *account == "" || *name == "" || *alias == "" {
			return domain.NewError(domain.CodeInvalidArgument, "profiles create requires --account, --name, and --alias")
		}
		return runRPCCommand(*root, "profiles.create", domain.CapabilityClientAdmin,
			map[string]any{"account_id": *account, "name": *name, "alias": *alias}, nil, true, *jsonOutput, stdout)
	case "list":
		body := map[string]any{"limit": *limit}
		if *account != "" {
			body["account_id"] = *account
		}
		if *cursor != "" {
			body["cursor"] = *cursor
		}
		return runRPCCommand(*root, "profiles.list", domain.CapabilityMetadataRead, body, nil, false, *jsonOutput, stdout)
	case "show":
		return runRPCCommand(*root, "profiles.show", domain.CapabilityMetadataRead, map[string]any{"target": target}, nil, false, *jsonOutput, stdout)
	case "validate":
		return runRPCCommand(*root, "profiles.validate", domain.CapabilityClientAdmin, map[string]any{"target": target}, nil, false, *jsonOutput, stdout)
	case "update":
		if *revision < 1 {
			return domain.NewError(domain.CodeInvalidArgument, "profiles update requires --if-revision")
		}
		body := map[string]any{"target": target, "expected_revision": *revision}
		visited := visitedFlags(flags)
		if visited["name"] {
			body["name"] = *name
		}
		if visited["alias"] {
			body["alias"] = *alias
		}
		if len(body) == 2 {
			return domain.NewError(domain.CodeInvalidArgument, "profiles update requires a changed field")
		}
		return runRPCCommand(*root, "profiles.update", domain.CapabilityClientAdmin, body, nil, true, *jsonOutput, stdout)
	case "disable", "enable":
		if *revision < 1 {
			return domain.NewError(domain.CodeInvalidArgument, "profile state change requires --if-revision")
		}
		return runRPCCommand(*root, "profiles."+action, domain.CapabilityClientAdmin,
			map[string]any{"target": target, "expected_revision": *revision}, nil, true, *jsonOutput, stdout)
	case "delete":
		if *revision < 1 || !*confirm {
			return domain.NewError(domain.CodeInvalidArgument, "profiles delete requires --if-revision and --confirm")
		}
		return runRPCCommand(*root, "profiles.delete", domain.CapabilityClientAdmin,
			map[string]any{"target": target, "expected_revision": *revision}, nil, true, *jsonOutput, stdout)
	default:
		return domain.NewError(domain.CodeMethodNotFound, "profiles command is not available")
	}
}

func runProviderTarget(method string, args []string, stdout, stderr *os.File) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return domain.NewError(domain.CodeInvalidArgument, "an explicit @alias is required")
	}
	target, flagArgs := args[0], args[1:]
	flags := flag.NewFlagSet(method, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	jsonOutput := flags.Bool("json", false, "JSON output")
	browserProfile := flags.String("browser-profile", "", "operator browser profile label")
	if err := flags.Parse(flagArgs); err != nil || *root == "" || len(flags.Args()) != 0 {
		return domain.NewError(domain.CodeInvalidArgument, "provider command arguments are invalid")
	}
	body := map[string]any{"target": target}
	if *browserProfile != "" {
		body["browser_profile"] = *browserProfile
	}
	return runRPCCommand(*root, method, domain.CapabilityClientAdmin, body, nil, false, *jsonOutput, stdout)
}

func runRegistryUsage(args []string, stdout, stderr *os.File) error {
	flags := flag.NewFlagSet("usage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private Device root")
	profile := flags.String("profile", "", "optional @alias")
	refresh := flags.Bool("refresh", false, "request live Provider refresh")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil || *root == "" || len(flags.Args()) != 0 {
		return domain.NewError(domain.CodeInvalidArgument, "usage requires --root")
	}
	method, capability := "usage.list", domain.CapabilityMetadataRead
	if *refresh {
		method, capability = "usage.refresh", domain.CapabilityClientAdmin
	}
	body := map[string]any{}
	if *profile != "" {
		body["profile"] = *profile
	}
	return runRPCCommand(*root, method, capability, body, nil, false, *jsonOutput, stdout)
}

func targetAndFlags(action string, args []string, needsTarget map[string]bool) (string, []string, error) {
	if !needsTarget[action] {
		return "", args, nil
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", nil, domain.NewError(domain.CodeInvalidArgument, action+" requires an account/profile target")
	}
	return args[0], args[1:], nil
}

func visitedFlags(flags *flag.FlagSet) map[string]bool {
	result := map[string]bool{}
	flags.Visit(func(item *flag.Flag) { result[item.Name] = true })
	return result
}
