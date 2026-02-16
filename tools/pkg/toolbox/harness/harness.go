// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package harness

import "fmt"

func Run(args []string) error {
	fmt.Println("Harness (prove) running with args:", args)
	return nil
}
