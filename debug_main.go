package main

import (
	"blade_engine/engine"
	"fmt"
	"os"
	"path/filepath"
)

func TestDebugMain() {
	if len(os.Args) < 2 {
		fmt.Println("usage: debug_main <template-path>")
		os.Exit(2)
	}
	root := filepath.Clean("templates")
	compiler := engine.NewCompilerWithOptions(root, "blade", nil)
	compiled, err := compiler.Compile(os.Args[1])
	if err != nil {
		fmt.Printf("compile error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(compiled)
}
