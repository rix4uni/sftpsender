package banner

import (
	"fmt"
)

// prints the version message
const version = "v0.0.3"

func PrintVersion() {
	fmt.Printf("Current sftpsender version %s\n", version)
}

// Prints the Colorful banner
func PrintBanner() {
	banner := `
          ____ __                                 __           
   _____ / __// /_ ____   _____ ___   ____   ____/ /___   _____
  / ___// /_ / __// __ \ / ___// _ \ / __ \ / __  // _ \ / ___/
 (__  )/ __// /_ / /_/ /(__  )/  __// / / // /_/ //  __// /    
/____//_/   \__// .___//____/ \___//_/ /_/ \__,_/ \___//_/     
               /_/
`
	fmt.Printf("%s\n%50s\n\n", banner, "Current sftpsender version "+version)
}
