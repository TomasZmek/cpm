package main

// Modules compiled into CPM.
//
// A module registers itself with sdk.Register from its init function, so a
// blank import is all it needs (see sdk/sdk.go and docs/modules.md). Which
// modules are active is decided at runtime in Settings → Modules and stored
// in caddy-config/modules.json.
//
// Modules can also be added at build time without editing this file:
//
//	docker build --build-arg CPM_MODULES="github.com/you/cpm-module@v0.1.0" .
//
// Example:
//
//	import _ "github.com/TomasZmek/cpm-dockhand"
