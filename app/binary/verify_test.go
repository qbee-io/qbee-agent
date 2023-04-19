package binary

import (
	"os"
	"testing"
)

func TestVerify(t *testing.T) {
	const (
		content   = "test"
		digest    = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
		signature = "MEYCIQCbbgslVegJXFczWSLP0lFKflbXdOtgMWslm/AQy1nIRQIhALjSztLgg4JltImIy33adWkH3WHS3+5F/aI1jk5/KrB8"
	)

	tests := []struct {
		name     string
		content  string
		metadata Metadata
		wantErr  string
	}{
		{
			name:    "digest mismatch",
			content: content,
			metadata: Metadata{
				Digest: "123",
			},
			wantErr: "digest mismatch: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08 != 123",
		},
		{
			name:    "invalid signature",
			content: content,
			metadata: Metadata{
				Digest:    digest,
				Signature: "123",
			},
			wantErr: "cannot decode signature: illegal base64 data at input byte 0",
		},
		{
			name:    "signature mismatch",
			content: "modified-content",
			metadata: Metadata{
				Digest:    "a28b2c91a9ffbc96aebbf06d5ff3f022a2f6524fac4bffa46e2d3b4dd7e9b153",
				Signature: signature,
			},
			wantErr: "signature mismatch",
		},
		{
			name:    "valid",
			content: content,
			metadata: Metadata{
				Digest:    digest,
				Signature: signature,
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create test file
			fp, err := os.CreateTemp(t.TempDir(), "")
			if err != nil {
				t.Fatalf("cannot create temp file: %v", err)
			}

			if err = fp.Chmod(nonExecutableFileMode); err != nil {
				t.Fatalf("cannot set permissions: %v", err)
			}
			defer fp.Close()

			if _, err = fp.WriteString(tt.content); err != nil {
				t.Fatalf("cannot write to file: %v", err)
			}

			if err = fp.Close(); err != nil {
				t.Fatalf("cannot close file: %v", err)
			}

			err = Verify(fp.Name(), &tt.metadata)

			// check file permissions
			info, statErr := os.Stat(fp.Name())
			if statErr != nil {
				t.Fatalf("cannot stat file: %v", statErr)
			}

			// make sure that file is not executable on error
			if err != nil && info.Mode() == executableFileMode {
				t.Fatalf("file is executable on error: %v", err)
			}

			// make sure that file is executable on success
			if err == nil && info.Mode() != executableFileMode {
				t.Fatalf("file is not executable on success: %v", err)
			}

			// check expected error
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Verify() expected error = %v", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("Verify() error = %s, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Verify() unexpected error = %v", err)
			}
		})
	}
}
