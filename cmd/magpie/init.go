package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"

	"github.com/ChamberZ40/magpie/config"
	"github.com/ChamberZ40/magpie/core"
	"github.com/ChamberZ40/magpie/daemon"
)

// inInitWizard silences the "Next: run magpie" footer of the setup commands
// when `magpie init` drives them, since init goes on to start magpie itself.
var inInitWizard bool

// runInit walks a new user from nothing to a running bridge: agent, work_dir,
// chat app, recommended settings, then starting it.
func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	configFile := fs.String("config", "", "path to config file")
	project := fs.String("project", "", "project to set up (optional if only one project)")
	assumeYes := fs.Bool("yes", false, "accept every default without asking")
	_ = fs.Parse(args)

	initConfigPath(*configFile)
	path := config.ConfigPath
	p := newPrompter(os.Stdin, os.Stdout, *assumeYes)

	fmt.Println("magpie init: a few questions to get your coding agent into your chat app.")
	fmt.Println()
	bootstrapConfigForSetup(path)

	name := initPickProject(p, *project)
	fmt.Printf("Setting up project %q in %s\n\n", name, path)

	initStepAgent(p, path, name)
	initStepWorkDir(p, path, name)
	initStepPlatform(p, path, name)
	initStepSettings(p, path, name)
	initStepStart(p, path)
}

func initFail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func initLoadProject(path, name string) config.ProjectConfig {
	cfg, err := config.LoadPermissive(path)
	if err != nil {
		initFail("load config: %v", err)
	}
	for _, proj := range cfg.Projects {
		if proj.Name == name {
			return proj
		}
	}
	initFail("project %q not found in %s", name, path)
	return config.ProjectConfig{}
}

func initPickProject(p *prompter, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	projects, err := config.ListProjects()
	if err != nil {
		initFail("%v", err)
	}
	switch len(projects) {
	case 0:
		initFail("no project in %s", config.ConfigPath)
	case 1:
		return projects[0]
	}
	sort.Strings(projects)
	return projects[p.choose("Which project do you want to set up?", projects, 0)]
}

func initStepAgent(p *prompter, path, name string) {
	fmt.Println("── 1/5 Coding agent")
	current := initLoadProject(path, name).Agent.Type
	agents := detectAgents(core.ListRegisteredAgents(), exec.LookPath)
	if len(agents) == 0 {
		fmt.Printf("Keeping agent %q.\n\n", current)
		return
	}
	labels := make([]string, len(agents))
	for i, a := range agents {
		status := "not installed"
		if a.Found {
			status = "installed"
		}
		labels[i] = fmt.Sprintf("%s (%s: %s)", a.Label, a.Binary, status)
	}
	picked := agents[p.choose("Which agent should answer in chat?", labels, defaultAgentIndex(agents, current))]
	if picked.Type != current {
		if err := config.SaveAgentType(name, picked.Type); err != nil {
			initFail("save agent type: %v", err)
		}
	}
	if !picked.Found {
		fmt.Printf("⚠️  %q is not on your PATH. Install %s before starting magpie.\n", picked.Binary, picked.Label)
	}
	fmt.Printf("magpie runs %s with your existing login. If you have never logged in, run `%s` once first.\n\n",
		picked.Binary, picked.Binary)
}

func initStepWorkDir(p *prompter, path, name string) {
	fmt.Println("── 2/5 Project directory")
	def, _ := os.Getwd()
	if current, _ := initLoadProject(path, name).Agent.Options["work_dir"].(string); current != "" && current != placeholderWorkDir {
		def = current
	}
	for {
		dir, err := resolveWorkDir(p.ask("Which directory should the agent work in?", def))
		if err == nil {
			if err := config.SaveAgentWorkDir(name, dir); err != nil {
				initFail("save work_dir: %v", err)
			}
			fmt.Printf("work_dir = %s\n\n", dir)
			return
		}
		fmt.Println(err)
		if p.assumeYes {
			initFail("work_dir: %v", err)
		}
	}
}

// initSetupChoices pairs each setup command with the function that runs it.
func initSetupChoices() (labels []string, run []func(args []string)) {
	for _, cmd := range setupCommands() {
		switch cmd {
		case "magpie feishu setup":
			labels = append(labels, "Feishu / Lark (scan a QR code, a bot is created for you)")
			run = append(run, func(a []string) { runFeishuSetup(a, feishuSetupModeAuto) })
		case "magpie weixin setup":
			labels = append(labels, "Weixin, personal WeChat (scan a QR code to link it)")
			run = append(run, func(a []string) { runWeixinSetup(a, weixinSetupModeAuto) })
		}
	}
	return labels, run
}

func initStepPlatform(p *prompter, path, name string) {
	fmt.Println("── 3/5 Chat app")
	existing := initLoadProject(path, name).Platforms
	if len(existing) > 0 {
		types := make([]string, len(existing))
		for i, pl := range existing {
			types[i] = pl.Type
		}
		fmt.Printf("Already connected: %v\n", types)
		if !p.confirm("Connect another chat app?", false) {
			fmt.Println()
			return
		}
	}
	labels, run := initSetupChoices()
	if len(labels) == 0 {
		initFail("this build has no chat app with a setup command; see magpie config example")
	}
	if p.assumeYes && len(existing) == 0 {
		initFail("connecting a chat app needs a QR scan; run magpie init without --yes")
	}
	i := p.choose("Which chat app?", labels, 0)
	inInitWizard = true
	run[i]([]string{"--config", path, "--project", name})
	inInitWizard = false
	fmt.Println()
}

func initStepSettings(p *prompter, path, name string) {
	fmt.Println("── 4/5 Recommended settings (all on by default; answer n to turn one off)")
	var types []string
	for _, pl := range initLoadProject(path, name).Platforms {
		types = append(types, pl.Type)
	}
	for _, s := range recommendedSettings(types, runtime.GOOS) {
		if err := s.apply(name, p.confirm(s.label+"?", true)); err != nil {
			initFail("save setting: %v", err)
		}
	}
	fmt.Println()
}

func initStepStart(p *prompter, path string) {
	fmt.Println("── 5/5 Start")
	if _, err := config.Load(path); err != nil {
		fmt.Print(configLoadErrorHint(path, err))
		os.Exit(1)
	}
	options := []string{
		"Run in the background as a service (keeps running after you close this terminal)",
		"Run in this terminal now",
		"Not now",
	}
	switch p.choose("How should magpie run?", options, 0) {
	case 0:
		initStartService(path)
	case 1:
		initStartForeground(path)
	default:
		fmt.Println("Done. Start it later with: magpie")
	}
}

func initStartService(path string) {
	mgr, err := daemon.NewManager()
	if err != nil {
		initFail("%v", err)
	}
	if st, _ := mgr.Status(); st != nil && st.Installed {
		if err := mgr.Restart(); err != nil {
			initFail("restart service: %v", err)
		}
		fmt.Println("Service restarted with the new config.")
	} else {
		daemonInstall([]string{"--config", path})
	}
	fmt.Println("Message your bot to try it. Follow the logs with: magpie daemon logs -f")
}

func initStartForeground(path string) {
	exe, err := os.Executable()
	if err != nil {
		initFail("find magpie binary: %v", err)
	}
	fmt.Println("Starting magpie. Message your bot to try it; press Ctrl+C to stop.")
	cmd := exec.Command(exe, "--config", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		initFail("run magpie: %v", err)
	}
}
