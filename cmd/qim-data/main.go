package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"qim-data/internal/config"
	"qim-data/internal/croc"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "setup":
		return runSetup(args[1:])
	case "send":
		return runSend(args[1:])
	case "receive":
		return runReceive(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func printUsage() {
	fmt.Print(`qim-data - DTU/MAX IV transfer wrapper for croc

Usage:
  qim-data setup [flags]
  qim-data send [flags] <file-or-folder> [more files...]
  qim-data receive [flags] [code]
  qim-data doctor [flags]

Commands:
  setup    Configure relay, relay secret, and croc path.
  send     Send files/folders using default Qim relay settings.
  receive  Receive transfers; optionally provide the code.
  doctor   Validate local setup and relay reachability.

Examples:
  qim-data setup --pass-file ~/.config/qim-data/relay.pass
  qim-data send ./dataset.zarr
  qim-data receive
  qim-data receive 1234-word-code
  qim-data doctor
`)
}

func runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	relay := fs.String("relay", config.DefaultRelay, "Relay endpoint host:port")
	pass := fs.String("pass", "", "Relay password (prefer --pass-file)")
	passFile := fs.String("pass-file", "", "Path to file containing relay password")
	crocPath := fs.String("croc-path", "", "Path to croc binary (optional)")
	nonInteractive := fs.Bool("non-interactive", false, "Fail instead of prompting when password is missing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("setup does not accept positional args")
	}

	cfg := config.Default()
	cfg.Relay = strings.TrimSpace(*relay)
	if cfg.Relay == "" {
		return errors.New("--relay cannot be empty")
	}

	relayPass := strings.TrimSpace(*pass)
	if relayPass == "" && *passFile != "" {
		b, err := os.ReadFile(*passFile)
		if err != nil {
			return fmt.Errorf("read --pass-file %s: %w", *passFile, err)
		}
		relayPass = strings.TrimSpace(string(b))
	}
	if relayPass == "" && !*nonInteractive {
		entered, err := prompt("Relay password (input visible): ")
		if err != nil {
			return err
		}
		relayPass = strings.TrimSpace(entered)
	}
	if relayPass == "" {
		return errors.New("relay password is empty; provide --pass, --pass-file, or interactive input")
	}
	cfg.RelayPass = relayPass

	if *crocPath != "" {
		cfg.CrocPath = strings.TrimSpace(*crocPath)
	} else {
		if p, err := croc.ResolveBinary(cfg, ""); err == nil {
			cfg.CrocPath = p
		}
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	cfgPath, err := config.Path()
	if err != nil {
		return err
	}
	fmt.Printf("Saved config: %s\n", cfgPath)
	fmt.Printf("Relay: %s\n", cfg.Relay)
	if cfg.CrocPath != "" {
		fmt.Printf("Croc binary: %s\n", cfg.CrocPath)
	}
	fmt.Println("Next: run `qim-data doctor`")
	return nil
}

func runSend(args []string) error {
	head, passthrough := splitArgsOnDoubleDash(args)

	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	relayOverride := fs.String("relay", "", "Relay endpoint host:port")
	crocPathOverride := fs.String("croc-path", "", "Path to croc binary")
	code := fs.String("code", "", "Transfer code override (optional)")
	if err := fs.Parse(head); err != nil {
		return err
	}

	paths := fs.Args()
	if len(paths) == 0 {
		return errors.New("send requires at least one file/folder")
	}

	cfg, err := requireConfig()
	if err != nil {
		return err
	}

	relay := chooseRelay(cfg.Relay, strings.TrimSpace(*relayOverride))
	crocPath, err := croc.ResolveBinary(cfg, strings.TrimSpace(*crocPathOverride))
	if err != nil {
		return err
	}

	crocArgs := []string{
		"--relay", relay,
		"--pass", cfg.RelayPass,
		"send",
	}
	if trimmed := strings.TrimSpace(*code); trimmed != "" {
		crocArgs = append(crocArgs, "--code", trimmed)
	}
	crocArgs = append(crocArgs, passthrough...)
	crocArgs = append(crocArgs, paths...)

	return croc.Run(crocPath, crocArgs, nil)
}

func runReceive(args []string) error {
	head, passthrough := splitArgsOnDoubleDash(args)

	fs := flag.NewFlagSet("receive", flag.ContinueOnError)
	relayOverride := fs.String("relay", "", "Relay endpoint host:port")
	crocPathOverride := fs.String("croc-path", "", "Path to croc binary")
	outDir := fs.String("out", "", "Output directory")
	if err := fs.Parse(head); err != nil {
		return err
	}

	var code string
	if fs.NArg() > 1 {
		return errors.New("receive accepts at most one code argument")
	}
	if fs.NArg() == 1 {
		code = strings.TrimSpace(fs.Arg(0))
	}

	cfg, err := requireConfig()
	if err != nil {
		return err
	}

	relay := chooseRelay(cfg.Relay, strings.TrimSpace(*relayOverride))
	crocPath, err := croc.ResolveBinary(cfg, strings.TrimSpace(*crocPathOverride))
	if err != nil {
		return err
	}

	crocArgs := []string{
		"--relay", relay,
		"--pass", cfg.RelayPass,
	}
	if out := strings.TrimSpace(*outDir); out != "" {
		crocArgs = append(crocArgs, "--out", out)
	}
	crocArgs = append(crocArgs, passthrough...)

	extraEnv := map[string]string{}
	if code != "" {
		// Using CROC_SECRET avoids classic-mode issues on Linux/macOS.
		extraEnv["CROC_SECRET"] = code
	}

	return croc.Run(crocPath, crocArgs, extraEnv)
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	relayOverride := fs.String("relay", "", "Relay endpoint host:port")
	crocPathOverride := fs.String("croc-path", "", "Path to croc binary")
	timeout := fs.Duration("timeout", 3*time.Second, "Relay dial timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("doctor does not accept positional args")
	}

	cfg, cfgErr := config.Load()
	hasConfig := cfgErr == nil
	if cfgErr != nil && !errors.Is(cfgErr, os.ErrNotExist) {
		return cfgErr
	}
	if !hasConfig {
		cfg = config.Default()
	}

	failed := false

	if hasConfig {
		printCheck(true, "config file", "found")
	} else {
		printCheck(false, "config file", "missing (run `qim-data setup`)")
		failed = true
	}

	relay := chooseRelay(cfg.Relay, strings.TrimSpace(*relayOverride))
	relay = withDefaultPort(relay, "9009")
	if relay == "" {
		printCheck(false, "relay", "empty")
		failed = true
	} else {
		printCheck(true, "relay", relay)
	}

	if strings.TrimSpace(cfg.RelayPass) == "" {
		printCheck(false, "relay password", "empty in config")
		failed = true
	} else {
		printCheck(true, "relay password", "configured")
	}

	crocPath, err := croc.ResolveBinary(cfg, strings.TrimSpace(*crocPathOverride))
	if err != nil {
		printCheck(false, "croc binary", err.Error())
		failed = true
	} else {
		printCheck(true, "croc binary", crocPath)

		version, vErr := croc.Version(crocPath)
		if vErr != nil {
			printCheck(false, "croc version", vErr.Error())
			failed = true
		} else {
			major, ok := croc.ParseMajor(version)
			if !ok {
				printCheck(false, "croc version", "could not parse: "+version)
				failed = true
			} else if major < 10 {
				printCheck(false, "croc version", version+" (requires v10+)")
				failed = true
			} else {
				printCheck(true, "croc version", version)
			}
		}
	}

	if err := croc.CheckRelayDial(relay, *timeout); err != nil {
		printCheck(false, "relay reachability", err.Error())
		failed = true
	} else {
		printCheck(true, "relay reachability", "tcp dial ok")
	}

	if failed {
		return errors.New("doctor checks failed")
	}

	fmt.Println("All checks passed.")
	return nil
}

func requireConfig() (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, errors.New("missing config (run `qim-data setup` first)")
		}
		return cfg, err
	}
	if strings.TrimSpace(cfg.RelayPass) == "" {
		return cfg, errors.New("relay password not configured (run `qim-data setup`)")
	}
	if cfg.Relay == "" {
		cfg.Relay = config.DefaultRelay
	}
	return cfg, nil
}

func chooseRelay(configRelay, override string) string {
	if override != "" {
		return override
	}
	if configRelay != "" {
		return configRelay
	}
	return config.DefaultRelay
}

func splitArgsOnDoubleDash(args []string) ([]string, []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func withDefaultPort(relay, defaultPort string) string {
	relay = strings.TrimSpace(relay)
	if relay == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(relay); err == nil {
		return relay
	}

	if strings.Contains(relay, ":") && !strings.HasPrefix(relay, "[") {
		// Could be ipv6 without brackets. Use as-is and let dial decide.
		return relay
	}
	return net.JoinHostPort(relay, defaultPort)
}

func printCheck(ok bool, check, detail string) {
	state := "OK"
	if !ok {
		state = "FAIL"
	}
	fmt.Printf("[%s] %-18s %s\n", state, check, detail)
}

func prompt(message string) (string, error) {
	fmt.Print(message)
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}
