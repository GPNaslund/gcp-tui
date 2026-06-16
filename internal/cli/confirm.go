package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// typedConfirm requires the operator to type the env name before a protected
// tunnel starts.
func typedConfirm(name string) bool {
	fmt.Printf("PROTECTED environment. Type %q to continue: ", name)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line) == name
}
