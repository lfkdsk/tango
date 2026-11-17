package main

type Options struct {
	Namespace, Subsystem, Name, Help string
	ConstLabels                      map[string]string
}

//go:noinline
func name(ns, sub, metric string) string { return ns + "_" + sub + "_" + metric }

//go:noinline
func valid(metric, help string, variables []string, constants map[string]string) int {
	var mismatch int
	if constants != nil {
		mismatch |= 1
	}
	if variables != nil {
		mismatch |= 2
	}
	if len(metric) != 5 {
		mismatch |= 4
	}
	if len(help) != 4 {
		mismatch |= 8
	}
	return mismatch
}

//go:noinline
func gauge(opts Options) int {
	return valid(name(opts.Namespace, opts.Subsystem, opts.Name), opts.Help, nil, opts.ConstLabels)
}

func main() {
	if mismatch := gauge(Options{Namespace: "n", Subsystem: "s", Name: "x", Help: "help"}); mismatch != 0 {
		panic(mismatch)
	}
}
