package main

import (
	"flag"
	"fmt"

	"github.com/jamessynge/transit_tools/nextbus/configfetch"
)

func main() {
	flag.Parse()

	dir1 := `C:\temp\nextbus_fetcher.M\mbta\config\2014\11\05\2014-11-05_0911`
	dir2 := `C:\temp\nextbus_fetcher.S\mbta\config\2014\12\06\2014-12-06_1300`

	fmt.Println("Comparing:", dir1)
	fmt.Println("  Against:", dir2)

	eq, err := configfetch.CompareConfigDirs(dir1, dir2)

	if eq && err == nil {
		// Can get rid of last dir as it is identical for our purposes.
		fmt.Println("Configurations are the same")
	} else if err != nil {
		fmt.Println("Errors while comparing directories:\n", err)
	} else {
		fmt.Println("Configurations are different")
	}
}
