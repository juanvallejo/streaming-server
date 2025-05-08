package embed

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func Embed(targetFilename, embedCodeSourceFilename string, out io.Writer) error {
	targetFile, err := os.Open(targetFilename)
	if err != nil {
		return err
	}

	targetFileData, err := ioutil.ReadAll(targetFile)
	if err != nil {
		return err
	}

	embedCodeFile, err := os.Open(embedCodeSourceFilename)
	if err != nil {
		return err
	}

	embedCodeFileData, err := ioutil.ReadAll(embedCodeFile)
	if err != nil {
		return err
	}

	needle := "<head>"
	contents := string(targetFileData)
	idx := strings.Index(contents, needle)
	if idx < 0 {
		return fmt.Errorf("no %q tag found; unable to embed analytics information", needle)
	}

	modified := contents[:idx+len(needle)] + string(embedCodeFileData)
	modified = modified + contents[idx+len(needle):]
	fmt.Fprintf(out, "%s\n", modified)
	return nil
}
