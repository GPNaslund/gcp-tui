package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

var (
	flagJSON   bool
	flagDryRun bool
	flagYes    bool
)

// out is the writer all CLI output goes to. Tests replace it with a buffer.
var out io.Writer = os.Stdout

// emit renders data as indented JSON when --json is set, otherwise calls human.
func emit(data any, human func() error) error {
	if flagJSON {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("json encode: %w", err)
		}
		_, err = fmt.Fprintln(out, string(b))
		return err
	}
	return human()
}
