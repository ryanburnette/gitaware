package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/ryanburnette/gitaware/internal/model"
)

// JSON writes the report as indented JSON.
func JSON(w io.Writer, report model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}
