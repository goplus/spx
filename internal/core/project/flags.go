package project

import "flag"

type CommandLineEffects struct {
	Verbose  bool
	ShowHelp bool
}

func ParseCommandLineFlags(fs *flag.FlagSet, args []string, conf *Config) (CommandLineEffects, error) {
	var effects CommandLineEffects
	if conf.DontParseFlags {
		return effects, nil
	}

	verbose := fs.Bool("v", false, "print verbose information")
	fullscreen := fs.Bool("f", false, "full screen")
	help := fs.Bool("h", false, "show help information")
	fullscreen2 := fs.Bool("fullscreen", false, "server mode")

	fs.String("controller", "", "controller's name")
	fs.Bool("servermode", false, "server mode")
	fs.String("serveraddr", "", "server address")
	fs.Bool("nomap", false, "server mode")
	fs.Bool("debugweb", false, "server mode")
	fs.String("gdextpath", "", "godot extension path")
	fs.String("write-movie", "", "movie mode")

	fs.String("path", "", "gdspx project path")
	fs.Bool("e", false, "editor mode")
	fs.Bool("headless", false, "Headless Mode")
	fs.Bool("remote-debug", false, "remote Debug Mode")
	fs.Bool("no-header", false, "disable engine's header output")

	if err := fs.Parse(args); err != nil {
		return effects, err
	}

	effects.Verbose = *verbose
	effects.ShowHelp = *help
	conf.FullScreen = conf.FullScreen || *fullscreen2 || *fullscreen
	return effects, nil
}
