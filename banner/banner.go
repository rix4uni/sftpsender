package banner

import (
	"fmt"
)

// prints the version message
const version = "v0.0.1"

func PrintVersion() {
	fmt.Printf("Current sender version %s\n", version)
}

// Prints the Colorful banner
func PrintBanner() {
	banner := `
                           __           
   _____ ___   ____   ____/ /___   _____
  / ___// _ \ / __ \ / __  // _ \ / ___/
 (__  )/  __// / / // /_/ //  __// /    
/____/ \___//_/ /_/ \__,_/ \___//_/
`
	fmt.Printf("%s\n%40s\n\n", banner, "Current sender version "+version)
}
