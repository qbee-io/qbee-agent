// Copyright 2023 qbee.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.qbee.io/agent/app/utils/assert"
)

func Test_renderTemplate(t *testing.T) {
	tests := []struct {
		name    string
		src     []byte
		wantDst []byte
		params  map[string]string
	}{
		{
			name:    "no tags",
			src:     []byte("no tags"),
			wantDst: []byte("no tags"),
		},
		{
			name:    "with tags",
			src:     []byte("tag1: {{tag1}}\ntag2: {{tag2}}\r\n"),
			params:  map[string]string{"tag1": "test-tag-1", "tag2": "test-tag-2"},
			wantDst: []byte("tag1: test-tag-1\ntag2: test-tag-2\r\n"),
		},
		{
			name:    "ends with new line",
			src:     []byte("no tags\n"),
			wantDst: []byte("no tags\n"),
		},
		{
			name:    "multi-line linux LF",
			src:     []byte("line 1\nline2\n"),
			wantDst: []byte("line 1\nline2\n"),
		},
		{
			name:    "multi-line windows CR LF",
			src:     []byte("line 1\r\nline2\r\n"),
			wantDst: []byte("line 1\r\nline2\r\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := bytes.NewBuffer(tt.src)
			dst := new(bytes.Buffer)

			if err := renderTemplate(src, tt.params, dst); err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			if !bytes.Equal(dst.Bytes(), tt.wantDst) {
				t.Fatalf("expected:\n%s\ngot:\n%s", tt.wantDst, dst.Bytes())
			}
		})
	}
}

func Test_renderTemplateLine(t *testing.T) {
	tests := []struct {
		name   string
		line   []byte
		params map[string]string

		want []byte
	}{
		{
			name:   "no tags",
			line:   []byte("test 1"),
			want:   []byte("test 1"),
			params: map[string]string{"tag": "1"},
		},
		{
			name:   "tag with spaces",
			line:   []byte("test {{ tag }}"),
			want:   []byte("test 1"),
			params: map[string]string{"tag": "1"},
		},
		{
			name:   "starts with tag",
			line:   []byte("{{ tag }} test"),
			want:   []byte("1 test"),
			params: map[string]string{"tag": "1"},
		},
		{
			name:   "tag without spaces",
			line:   []byte("test {{tag}}"),
			want:   []byte("test 1"),
			params: map[string]string{"tag": "1"},
		},
		{
			name:   "more than one tag per line",
			line:   []byte("test {{  tag1}} {{tag2   }}"),
			want:   []byte("test 1 2"),
			params: map[string]string{"tag1": "1", "tag2": "2"},
		},
		{
			name: "unclosed tag",
			line: []byte("test {{tag"),
			want: []byte("test {{tag"),
		},
		{
			name: "unknown tag",
			line: []byte("test {{tag}}"),
			want: []byte("test {{tag}}"),
		},
		{
			name:   "unknown properties and missing closing",
			line:   []byte("test {{tag1}} {{tag2}} {{valid}} {{ unclosed \n"),
			want:   []byte("test {{tag1}} {{tag2}} 1 {{ unclosed \n"),
			params: map[string]string{"valid": "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderTemplateLine(tt.line, tt.params)
			if err != nil {
				t.Fatalf("unexpected error = %v", err)
			}

			if !bytes.Equal(got, tt.want) {
				t.Errorf("got line = `%s`, want `%s`", got, tt.want)
			}
		})
	}
}

func Test_resolveDestinationPath(t *testing.T) {
	tempDir := t.TempDir()
	tests := []struct {
		name        string
		source      string
		destination string
		want        string
	}{
		{
			name:        "regular path",
			source:      "/test/source",
			destination: filepath.Join(tempDir, "destination"),
			want:        filepath.Join(tempDir, "destination"),
		},
		{
			name:        "dir path with trailing slash",
			source:      "/test/source",
			destination: fmt.Sprint(tempDir, "/"),
			want:        filepath.Join(tempDir, "source"),
		},
		{
			name:        "dir path without trailing slash",
			source:      "/test/source",
			destination: tempDir,
			want:        filepath.Join(tempDir, "source"),
		},
		{
			name:        "regular path",
			source:      "source",
			destination: tempDir,
			want:        filepath.Join(tempDir, "source"),
		},
		{
			name:        "from local source",
			source:      "file://source",
			destination: tempDir,
			want:        filepath.Join(tempDir, "source"),
		},
		{
			name:        "illegal path that shoould return empty string",
			source:      "source",
			destination: fmt.Sprintf("%s/notallowed/", tempDir),
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDestinationPath(tt.source, tt.destination)
			if err != nil {
				if got != tt.want {
					t.Fatalf("unexpected error = %v", err)
				}
			}
			if got != tt.want {
				t.Errorf("got = `%s`, want `%s`", got, tt.want)
			}
		})
	}
}

// Test_createFile_doesNotFollowSymlinks reproduces the symlink-following
// vulnerability in createFile() (file_manager.go). createFile is invoked by
// the root daemon to materialize files such as ~user/.ssh/authorized_keys
// (bundle_sshkeys.go) and the RAUC partial-download staging file
// (/tmp/.bundle.raucb.part). It opens the path with os.OpenFile using
// O_RDWR|O_CREATE[|O_TRUNC] and no O_NOFOLLOW, and its companion
// makeDirectories() uses symlink-following os.Stat. A local unprivileged
// user who controls a path component can therefore redirect a
// root-privileged write/chown through a symlink and clobber an arbitrary
// root-owned file (CWE-59).
//
// Each subtest below sets up a "victim" file the attacker should not be
// able to touch, plants a symlink representing the attacker-controlled
// component, then calls createFile() through the attacker-controlled
// path. A safe implementation must either fail or write to a fresh,
// non-symlinked location; in particular the victim file's contents must
// remain unchanged.
func Test_createFile_doesNotFollowSymlinks(t *testing.T) {
	t.Run("symlinked intermediate directory (authorized_keys variant)", func(t *testing.T) {
		tempDir := t.TempDir()

		// Victim directory + file, simulating /root/.ssh/authorized_keys
		// that already holds root's real key material.
		victimDir := filepath.Join(tempDir, "root_ssh")
		if err := os.Mkdir(victimDir, 0700); err != nil {
			t.Fatalf("setup: mkdir victim dir: %v", err)
		}
		victimFile := filepath.Join(victimDir, "authorized_keys")
		victimContent := []byte("root-original-authorized-keys\n")
		if err := os.WriteFile(victimFile, victimContent, 0600); err != nil {
			t.Fatalf("setup: write victim file: %v", err)
		}

		// Attacker's home directory (a directory the unprivileged user owns
		// — non-sticky, so fs.protected_symlinks does not mitigate).
		attackerHome := filepath.Join(tempDir, "alice_home")
		if err := os.Mkdir(attackerHome, 0700); err != nil {
			t.Fatalf("setup: mkdir attacker home: %v", err)
		}

		// Attacker plants ~alice/.ssh -> /root/.ssh (the victim dir).
		sshSymlink := filepath.Join(attackerHome, ".ssh")
		if err := os.Symlink(victimDir, sshSymlink); err != nil {
			t.Fatalf("setup: plant symlink: %v", err)
		}

		// The root daemon computes filepath.Join(user.HomeDirectory, ".ssh",
		// "authorized_keys") and hands it to createFile.
		targetPath := filepath.Join(sshSymlink, "authorized_keys")

		fcd := &fileCreateData{
			uid:        os.Geteuid(),
			gid:        os.Getegid(),
			bytesAvail: 1 << 30,
		}

		file, err := createFile(targetPath, fcd, sshAuthorizedKeysFilePermission, true)
		if file != nil {
			_, _ = file.Write([]byte("attacker-supplied-key\n"))
			_ = file.Close()
		}
		if err == nil {
			t.Errorf("createFile traversed a symlinked intermediate directory and returned no error; "+
				"expected it to refuse the symlinked .ssh path (target=%s)", targetPath)
		}

		got, readErr := os.ReadFile(victimFile)
		if readErr != nil {
			t.Fatalf("reading victim file: %v", readErr)
		}
		if !bytes.Equal(got, victimContent) {
			t.Errorf("victim file was overwritten through a symlinked intermediate directory.\n"+
				" path:     %s\n want:     %q\n got:      %q",
				victimFile, victimContent, got)
		}
	})

	t.Run("symlinked final component (RAUC /tmp staging variant)", func(t *testing.T) {
		tempDir := t.TempDir()

		// Victim file the attacker should not be able to overwrite/chown
		// (simulating e.g. /etc/shadow being targeted via the predictable
		// /tmp/.bundle.raucb.part staging path).
		victimFile := filepath.Join(tempDir, "victim_root_file")
		victimContent := []byte("important-root-owned-content\n")
		if err := os.WriteFile(victimFile, victimContent, 0600); err != nil {
			t.Fatalf("setup: write victim file: %v", err)
		}

		// Predictable staging location in an attacker-writable dir.
		stagingDir := filepath.Join(tempDir, "tmp")
		if err := os.Mkdir(stagingDir, 0777); err != nil {
			t.Fatalf("setup: mkdir staging dir: %v", err)
		}
		stagingPath := filepath.Join(stagingDir, ".bundle.raucb.part")

		// Attacker pre-creates the staging path as a symlink to the victim.
		if err := os.Symlink(victimFile, stagingPath); err != nil {
			t.Fatalf("setup: plant staging symlink: %v", err)
		}

		fcd := &fileCreateData{
			uid:        os.Geteuid(),
			gid:        os.Getegid(),
			bytesAvail: 1 << 30,
		}

		file, err := createFile(stagingPath, fcd, fileManagerDefaultFilePermission, true)
		if file != nil {
			_, _ = file.Write([]byte("server-bundle-bytes\n"))
			_ = file.Close()
		}
		if err == nil {
			t.Errorf("createFile opened a final-component symlink without O_NOFOLLOW and returned no error; "+
				"expected it to refuse the symlinked staging path (target=%s)", stagingPath)
		}

		got, readErr := os.ReadFile(victimFile)
		if readErr != nil {
			t.Fatalf("reading victim file: %v", readErr)
		}
		if !bytes.Equal(got, victimContent) {
			t.Errorf("victim file was truncated/overwritten through a final-component symlink.\n"+
				" path:     %s\n want:     %q\n got:      %q",
				victimFile, victimContent, got)
		}
	})
}

func Test_GetPartialDownloadFilePath(t *testing.T) {
	validDigestA := strings.Repeat("a", sha256.Size*2)
	validDigestB := strings.Repeat("b", sha256.Size*2)

	t.Run("includes the digest and stays in the destination directory", func(t *testing.T) {
		got := GetPartialDownloadFilePath("/var/lib/test.txt", validDigestA)
		assert.Equal(t, filepath.Dir(got), "/var/lib")
		assert.True(t, strings.HasSuffix(got, "."+validDigestA+".part"))
	})

	t.Run("different digests produce different paths", func(t *testing.T) {
		a := GetPartialDownloadFilePath("/var/lib/test.txt", validDigestA)
		b := GetPartialDownloadFilePath("/var/lib/test.txt", validDigestB)
		assert.NotEqual(t, a, b)
	})

	t.Run("different destinations produce paths where neither basename is a prefix of the other", func(t *testing.T) {
		a := filepath.Base(GetPartialDownloadFilePath("/var/lib/test.txt", validDigestA))
		b := filepath.Base(GetPartialDownloadFilePath("/var/lib/test.txt.other", validDigestA))
		assert.NotEqual(t, a, b)
		assert.False(t, strings.HasPrefix(a, b) || strings.HasPrefix(b, a))
	})

	t.Run("malformed digests are hashed into a safe fixed-width identifier", func(t *testing.T) {
		got := GetPartialDownloadFilePath("/var/lib/test.txt", "../../../etc/passwd")
		assert.Equal(t, filepath.Dir(got), "/var/lib")
		assert.False(t, strings.Contains(filepath.Base(got), "/"))
	})

	t.Run("file name stays within NAME_MAX", func(t *testing.T) {
		longName := strings.Repeat("a", 300)
		got := filepath.Base(GetPartialDownloadFilePath("/var/lib/"+longName, validDigestA))
		assert.False(t, len(got) > 255)
	})
}

func Test_downloadMetadataCompare_DestinationNameExceedingNameMax(t *testing.T) {
	tempDir := t.TempDir()

	contents := []byte("this is the contents of the file")
	srcPath := filepath.Join(tempDir, "source.txt")

	assert.NoError(t, os.WriteFile(srcPath, contents, 0600))
	// a valid destination name which would exceed NAME_MAX once the digest and suffix are appended
	dst := filepath.Join(tempDir, strings.Repeat("a", 250)+".txt")

	fileMetadata := &FileMetadata{
		Tags: map[string]string{fileDigestSHA256Tag: sha256Hex(contents)},
		Size: int64(len(contents)),
	}
	name := filepath.Base(GetPartialDownloadFilePath(dst, fileMetadata.Digest()))
	assert.False(t, len(name) > 255)

	srv := new(Service)

	created, err := srv.downloadMetadataCompare(context.Background(), "", "file://"+srcPath, dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)

	got, err := os.ReadFile(dst)
	assert.NoError(t, err)
	assert.Equal(t, contents, got)
}

func sha256Hex(contents []byte) string {
	digest := sha256.Sum256(contents)

	return hex.EncodeToString(digest[:])
}

func Test_downloadMetadataCompare_CompletePartialIsNotDownloadedAgain(t *testing.T) {
	tempDir := t.TempDir()
	dst := filepath.Join(tempDir, "file.txt")
	contents := []byte("this is the contents of the file")

	fileMetadata := &FileMetadata{
		Tags: map[string]string{fileDigestSHA256Tag: sha256Hex(contents)},
		Size: int64(len(contents)),
	}

	tmpDst := GetPartialDownloadFilePath(dst, fileMetadata.Digest())
	assert.NoError(t, os.WriteFile(tmpDst, contents, 0600))

	// a source which cannot be read at all - the fully downloaded partial file must be
	// verified and renamed without touching the source
	src := "file://" + filepath.Join(tempDir, "does-not-exist")
	srv := new(Service)

	created, err := srv.downloadMetadataCompare(context.Background(), "", src, dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)

	got, err := os.ReadFile(dst)
	assert.NoError(t, err)
	assert.Equal(t, contents, got)

	_, err = os.Stat(tmpDst)
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

func Test_downloadMetadataCompare_RemovesPartialDownloadsWithOtherDigest(t *testing.T) {
	tempDir := t.TempDir()

	srcPath := filepath.Join(tempDir, "source.txt")
	contents := []byte("this is the contents of the file")
	assert.NoError(t, os.WriteFile(srcPath, contents, 0600))

	dst := filepath.Join(tempDir, "file.txt")

	fileMetadata := &FileMetadata{
		Tags: map[string]string{fileDigestSHA256Tag: sha256Hex(contents)},
		Size: int64(len(contents)),
	}

	// a leftover partial download of the same destination, but of previous contents
	stalePartial := GetPartialDownloadFilePath(dst, sha256Hex([]byte("previous contents")))
	assert.NoError(t, os.WriteFile(stalePartial, []byte("previous"), 0600))

	// an unrelated file in the same directory which must be left alone
	unrelated := filepath.Join(tempDir, ".other.txt.abc.part")
	assert.NoError(t, os.WriteFile(unrelated, []byte("keep me"), 0600))

	srv := new(Service)

	created, err := srv.downloadMetadataCompare(context.Background(), "", "file://"+srcPath, dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)

	_, err = os.Stat(stalePartial)
	assert.True(t, errors.Is(err, fs.ErrNotExist))

	// this file should still exist
	_, err = os.Stat(unrelated)
	assert.NoError(t, err)

	got, err := os.ReadFile(dst)
	assert.NoError(t, err)
	assert.Equal(t, contents, got)
}
