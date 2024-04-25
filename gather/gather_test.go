package gather

import (
	"context"
	"testing"
)

func TestGather(t *testing.T) {
	ctx := context.Background()
	t.Run("SourceParseError", func(t *testing.T) {
		source := ":"
		destination := "/path/to/destination"

		_, err := Gather(ctx, source, destination)
		if err == nil {
			t.Error("expected an error, but got nil")
		}

		expectedErrorMessage := "parse \":\": missing protocol scheme"
		if err.Error() != expectedErrorMessage {
			t.Errorf("expected error message: %s, but got: %s", expectedErrorMessage, err.Error())
		}
	})

	t.Run("UnsupportedProtocol", func(t *testing.T) {
		source := "ftp://example.com/file.txt"
		destination := "/path/to/destination"

		_, err := Gather(ctx, source, destination)
		if err == nil {
			t.Error("expected an error, but got nil")
		}

		expectedErrorMessage := "unsupported source protocol: ftp"
		if err.Error() != expectedErrorMessage {
			t.Errorf("expected error message: %s, but got: %s", expectedErrorMessage, err.Error())
		}
	})
}
