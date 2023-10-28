package app

import (
	"log"

	"github.com/mikestefanello/hooks"
	"github.com/samber/do"
)

// HookBoot allows modules and services the ability to initialize and register dependencies
var HookBoot = hooks.NewHook[*do.Injector]("boot")

func init() {
	// See hooks logs
	hooks.SetLogger(func(format string, args ...any) {
		log.Printf(format+"\n", args...)
	})
}

// request all dependencies
func Boot() *do.Injector {
	HookBoot.Dispatch(do.DefaultInjector)

	// Log all dependencies
	d := do.DefaultInjector.ListProvidedServices()
	log.Printf("registered %d dependencies: %v", len(d), d)

	return do.DefaultInjector
}
