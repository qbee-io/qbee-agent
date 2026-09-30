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
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"go.qbee.io/agent/app/utils"
)

const (
	fileManagerDefaultDirectoryPermission = 0750
	fileManagerDefaultFilePermission      = 0640
)

// FileDistributionCacheDirectory is where the agent will download template files for processing.
const FileDistributionCacheDirectory = "file_distribution"

// SoftwareCacheDirectory is where the agent will download software packages to install.
const SoftwareCacheDirectory = "software"

// DockerContainerDirectory is where the agent will download docker related files.
const DockerContainerDirectory = "docker_containers"

// PodmanContainerDirectory is where the agent will download podman related files.
const PodmanContainerDirectory = "podman_containers"

// DockerComposeDirectory is where the agent will download docker-compose related files.
const DockerComposeDirectory = "docker_compose"

// FileMetadata is the metadata of a file.
type FileMetadata struct {
	MD5          string            `json:"md5"`
	LastModified int64             `json:"last_modified"`
	Tags         map[string]string `json:"tags,omitempty"`
	Size         int64             `json:"size,omitempty"`
}

// TemplateParameter defines a single parameter used to replace placeholder in a template.
type TemplateParameter struct {
	// Key of the parameter used in files.
	Key string `json:"key"`

	// Value of the parameter which will replace Key placeholders.
	Value string `json:"value"`
}

// ParametersMap returns TemplateParameters as map.
func templateParametersMap(templateParameters []TemplateParameter) map[string]string {
	parameters := make(map[string]string)

	for _, param := range templateParameters {
		parameters[param.Key] = param.Value
	}

	return parameters
}

const fileDigestSHA256Tag = "qbee_digest_sha256"

// SHA256 returns hex-encoded sha256 digest of the file (if present), otherwise an empty string.
func (md *FileMetadata) SHA256() string {
	if md.Tags == nil {
		return ""
	}

	return md.Tags[fileDigestSHA256Tag]
}

// Digest returns the digest identifying the expected contents of the file.
func (md *FileMetadata) Digest() string {
	if digest := md.SHA256(); digest != "" {
		return digest
	}

	return md.MD5
}

// downloadFile and return true when file was created. In case the right file already existed, return false.
func (srv *Service) downloadFile(ctx context.Context, label, src, dst string, file File) (bool, error) {
	var err error

	src = resolveParameters(ctx, src)
	dst = resolveParameters(ctx, dst)

	defer func() {
		if err != nil {
			ReportError(ctx, err, msgWithLabel(label, "Unable to download file %s to %s"), src, dst)
		}
	}()

	if !strings.HasPrefix(dst, "/") {
		err = fmt.Errorf("absolute file path required, got %s", dst)
		return false, err
	}

	var fileMetadata *FileMetadata
	if file.Digest != "" {
		fileMetadata = &FileMetadata{
			Tags: map[string]string{
				fileDigestSHA256Tag: file.Digest,
			},
			Size: file.Size,
		}
	} else if fileMetadata, err = srv.getFileMetadata(ctx, src); err != nil {
		return false, err
	}

	var fileReady bool
	fileReady, err = srv.downloadMetadataCompare(ctx, label, src, dst, fileMetadata)

	return fileReady, err
}

// downloadMetadataCompare downloads src to dst unless dst already matches fileMetadata.
//
// The download is staged in a private, agent-owned directory next to dst. Every operation on the
// partial file (open, write, verify, rename) goes through an os.Root anchored to dst's directory,
// so other users can't substitute files or directories by pathname (CWE-59, CWE-367).
func (srv *Service) downloadMetadataCompare(ctx context.Context, label, src, dst string, fileMetadata *FileMetadata) (bool, error) {
	if fileMetadata.Size < 0 {
		return false, fmt.Errorf("invalid file metadata for %s: file size must not be negative", src)
	}

	// check if file already exists and has the right contents
	if fileReady, err := isFileReady(dst, fileMetadata); err != nil || fileReady {
		return false, err
	}

	fileCreateData, err := determineFileCreateData(dst)
	if err != nil {
		return false, fmt.Errorf("error determining local fs data: %w", err)
	}

	if err = makeDirectories(dst, fileManagerDefaultDirectoryPermission, fileCreateData.uid, fileCreateData.gid); err != nil {
		return false, err
	}

	partialPath := GetPartialDownloadFilePath(dst, fileMetadata.Digest())
	partialDirName := filepath.Base(filepath.Dir(partialPath))
	partialName := filepath.Base(partialPath)

	dstRoot, err := openDirectoryAnchored(filepath.Dir(dst))
	if err != nil {
		return false, fmt.Errorf("error opening destination directory for %s: %w", dst, err)
	}
	defer func() { _ = dstRoot.Close() }()

	partialDir, err := openPrivatePartialDownloadDirectory(dstRoot, partialDirName)
	if err != nil {
		return false, err
	}
	defer func() { _ = partialDir.Close() }()

	if err = removeStalePartialDownloads(partialDir, partialName); err != nil {
		return false, err
	}

	partialFile, err := openPartialDownloadFile(partialDir, partialName)
	if err != nil {
		return false, fmt.Errorf("error opening partial download %s: %w", partialPath, err)
	}
	defer func() { _ = partialFile.Close() }()

	if err = srv.resumeDownload(ctx, src, partialFile, fileMetadata.Size, fileCreateData); err != nil {
		return false, fmt.Errorf("error downloading %s to %s: %w", src, partialPath, err)
	}

	if _, err = partialFile.Seek(0, io.SeekStart); err != nil {
		return false, err
	}

	valid, err := isFileReadyFd(partialFile, fileMetadata)
	if err != nil {
		return false, err
	}

	if !valid {
		_ = partialDir.Remove(partialName)
		_ = dstRoot.Remove(partialDirName)
		return false, fmt.Errorf("downloaded file is incomplete or has invalid contents")
	}

	if err = installPartialDownload(dstRoot, partialDir, partialFile, dst, fileCreateData); err != nil {
		return false, err
	}

	if err = dstRoot.Remove(partialDirName); err != nil {
		ReportWarning(ctx, err, "Unable to remove partial download directory %s", filepath.Dir(partialPath))
	}

	ReportInfo(ctx, nil, msgWithLabel(label, "Successfully downloaded file %s to %s"), src, dst)

	return true, nil
}

// openPartialDownloadFile opens (or creates) the regular file name within dir.
// O_RDWR makes opening a planted FIFO non-blocking on Linux; it is then rejected as non-regular.
func openPartialDownloadFile(dir *os.Root, name string) (*os.File, error) {
	// os.Root resolves symlinks itself, so a pre-existing symlink must be rejected explicitly.
	if info, err := dir.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	file, err := dir.OpenFile(name, os.O_RDWR|os.O_CREATE, fileManagerDefaultFilePermission)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("%s is not a regular file", name)
	}

	return file, nil
}

// resumeDownload appends the missing part of src to partialFile, starting over if it is larger than size.
func (srv *Service) resumeDownload(ctx context.Context, src string, partialFile *os.File, size int64, fileCreateData *fileCreateData) error {
	info, err := partialFile.Stat()
	if err != nil {
		return err
	}

	offset := info.Size()
	// requesting bytes beyond a complete file would fail with HTTP 416
	if offset == size {
		return nil
	}

	if offset > size {
		if err = partialFile.Truncate(0); err != nil {
			return err
		}
		offset = 0
	}

	// refresh fileCreateData.bytesAvail before checking disk space
	if err = fileCreateData.refreshBytesAvail(); err != nil {
		return err
	}

	remaining := size - offset
	if fileCreateData.bytesAvail < freeDiskOverhead || remaining > fileCreateData.bytesAvail-freeDiskOverhead {
		return fmt.Errorf("not enough disk space: need %d bytes plus %d bytes overhead, have %d bytes", remaining, freeDiskOverhead, fileCreateData.bytesAvail)
	}

	srcFile, err := srv.getFile(ctx, src, offset)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	if _, err = partialFile.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	_, err = io.CopyN(partialFile, srcFile, remaining)

	return err
}

// installPartialDownload sets ownership of the verified partialFile and renames it to dst.
//
// The rename is issued with renameat(2) against the already-open directory descriptors, so neither
// the staging directory nor dst's directory is resolved by name at rename time. That removes the
// window in which either could be swapped for a symlink or another directory after being verified
// (CWE-59, CWE-367).
func installPartialDownload(dstRoot, partialDir *os.Root, partialFile *os.File, dst string, fileCreateData *fileCreateData) error {
	if err := partialFile.Chown(fileCreateData.uid, fileCreateData.gid); err != nil {
		return fmt.Errorf("error setting owner on %s: %w", dst, err)
	}

	if err := partialFile.Chmod(fileManagerDefaultFilePermission); err != nil {
		return fmt.Errorf("error setting permissions on %s: %w", dst, err)
	}

	partialDirFile, err := partialDir.Open(".")
	if err != nil {
		return fmt.Errorf("error opening partial download directory for %s: %w", dst, err)
	}
	defer func() { _ = partialDirFile.Close() }()

	dstDirFile, err := dstRoot.Open(".")
	if err != nil {
		return fmt.Errorf("error opening destination directory for %s: %w", dst, err)
	}
	defer func() { _ = dstDirFile.Close() }()

	oldName := filepath.Base(partialFile.Name())

	err = unix.Renameat(int(partialDirFile.Fd()), oldName, int(dstDirFile.Fd()), filepath.Base(dst))
	if err != nil {
		return fmt.Errorf("error renaming partial download to %s: %w", dst, err)
	}

	return nil
}

const localFileSchema = "file://"

// getLocalFile returns file read-closer for a file on the local filesystem.
func getLocalFile(src string, offset int64) (io.ReadCloser, error) {
	fp, err := os.Open(strings.TrimPrefix(src, localFileSchema))
	if err != nil {
		return nil, err
	}

	if offset > 0 {
		if _, err := fp.Seek(offset, io.SeekStart); err != nil {
			_ = fp.Close()
			return nil, err
		}
	}

	return fp, nil
}

// getFile returns file reader for a file in file manager.
func (srv *Service) getFile(ctx context.Context, src string, offset int64) (io.ReadCloser, error) {
	if strings.HasPrefix(src, localFileSchema) {
		return getLocalFile(src, offset)
	}

	return srv.getFileFromAPI(ctx, src, offset)
}

// getFileMetadataFromLocal returns metadata for a file on the local filesystem.
func (srv *Service) getFileMetadataFromLocal(src string) (*FileMetadata, error) {
	fp, err := os.Open(strings.TrimPrefix(src, localFileSchema))
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %w", src, err)
	}

	defer func() { _ = fp.Close() }()

	var fileInfo os.FileInfo
	if fileInfo, err = fp.Stat(); err != nil {
		return nil, fmt.Errorf("error getting file metadata %s: %w", src, err)
	}

	digest := sha256.New()
	if _, err = io.Copy(digest, fp); err != nil {
		return nil, fmt.Errorf("error calculating file checksum %s: %w", src, err)
	}

	hexDigest := hex.EncodeToString(digest.Sum(nil))

	fileMetadata := &FileMetadata{
		LastModified: fileInfo.ModTime().Unix(),
		Tags: map[string]string{
			fileDigestSHA256Tag: hexDigest,
		},
		Size: fileInfo.Size(),
	}

	return fileMetadata, nil
}

// getFileMetadata returns metadata for a file in the file manager.
func (srv *Service) getFileMetadata(ctx context.Context, src string) (*FileMetadata, error) {
	if strings.HasPrefix(src, localFileSchema) {
		return srv.getFileMetadataFromLocal(src)
	}

	return srv.getFileMetadataFromAPI(ctx, src)
}

// downloadTemplateFile and execute - returns true if file template was executed and resulted in a new dst file.
func (srv *Service) downloadTemplateFile(
	ctx context.Context,
	label string,
	src string,
	dst string,
	file File,
	params map[string]string,
) (bool, error) {
	var err error

	src = resolveParameters(ctx, src)
	dst = resolveParameters(ctx, dst)

	for key := range params {
		params[key] = resolveParameters(ctx, params[key])
	}

	defer func() {
		if err != nil {
			ReportError(ctx, err, msgWithLabel(label, "Unable to render template file %s to %s."), src, dst)
		}
	}()

	var cacheSrc string

	if strings.HasPrefix(src, localFileSchema) {
		cacheSrc = strings.TrimPrefix(src, localFileSchema)
	} else {
		cacheSrc = filepath.Join(srv.cacheDirectory, FileDistributionCacheDirectory, src)

		if _, err = srv.downloadFile(ctx, label, src, cacheSrc, file); err != nil {
			return false, err
		}
	}

	var sha256digest string
	if sha256digest, err = calculateTemplateDigest(cacheSrc, params); err != nil {
		return false, err
	}

	fileMetadata := &FileMetadata{
		Tags: map[string]string{
			fileDigestSHA256Tag: sha256digest,
		},
	}

	var fileReady bool
	if fileReady, err = isFileReady(dst, fileMetadata); err != nil || fileReady {
		return false, err
	}

	fileCreateData, err := determineFileCreateData(dst)
	if err != nil {
		return false, fmt.Errorf("error determining local fs data: %w", err)
	}

	var srcFile io.ReadCloser
	if srcFile, err = os.Open(cacheSrc); err != nil {
		return false, fmt.Errorf("error opening template file %s: %w", cacheSrc, err)
	}

	defer func() { _ = srcFile.Close() }()

	var dstFile io.WriteCloser
	if dstFile, err = createFile(dst, fileCreateData, fileManagerDefaultFilePermission, true); err != nil {
		return false, err
	}

	defer func() { _ = dstFile.Close() }()

	if err = renderTemplate(srcFile, params, dstFile); err != nil {
		return false, err
	}

	ReportInfo(ctx, nil, msgWithLabel(label, "Successfully rendered template file %s to %s"), src, dst)

	return true, nil
}

const templateMaxTokenSize = 20 * 1024 * 1024 // 20MB for single-line-files

// renderTemplate to destination based on source reader and provided parameters.
func renderTemplate(src io.Reader, params map[string]string, dst io.Writer) error {
	scanner := bufio.NewScanner(src)
	scanner.Split(templateScanner)
	scanner.Buffer(nil, templateMaxTokenSize)

	lineNo := 0
	for scanner.Scan() {
		lineNo++

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("cannot read line %d: %w", lineNo, err)
		}

		line := scanner.Bytes()

		renderedLine, err := renderTemplateLine(line, params)
		if err != nil {
			return fmt.Errorf("error rendering line %d: %w", lineNo, err)
		}

		if _, err = dst.Write(renderedLine); err != nil {
			return fmt.Errorf("cannot write line %d: %w", lineNo, err)
		}
	}

	return nil
}

const (
	templateLeftDelimiter  = "{{"
	templateRightDelimiter = "}}"
)

// renderTemplateLine renders a single line of a template based on provided parameters.
func renderTemplateLine(line []byte, params map[string]string) ([]byte, error) {
	result := make([]byte, 0, len(line))

	for len(line) > 0 {
		leftDelimiterIndex := bytes.Index(line, []byte(templateLeftDelimiter))

		// if no tags found, add remainder of the line to the result and return
		if leftDelimiterIndex < 0 {
			result = append(result, line...)
			return result, nil
		}

		// copy bytes before the tag without any modifications
		result = append(result, line[:leftDelimiterIndex]...)

		// advance line to the start of the left delimiter
		line = line[leftDelimiterIndex:]

		// find right delimiter
		rightDelimiterIndex := bytes.Index(line, []byte(templateRightDelimiter))

		// if no closing delimiter found, add a warning and return remaining result
		if rightDelimiterIndex < 0 {
			result = append(result, line...)
			return result, nil
		}

		// extract tag name
		tag := bytes.TrimSpace(line[len(templateLeftDelimiter):rightDelimiterIndex])

		// identify tag value
		value, ok := params[string(tag)]
		if ok {
			// for known tags, replace with its value int the result
			result = append(result, []byte(value)...)
		} else {
			// otherwise copy the tag to the result
			result = append(result, line[:rightDelimiterIndex+len(templateRightDelimiter)]...)
		}

		// advance the line to after the right delimiter for further processing
		line = line[rightDelimiterIndex+len(templateRightDelimiter):]
	}

	return result, nil
}

// templateScanner works like bufio.ScanLines, but preserves new-line characters.
func templateScanner(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[0 : i+1], nil
	}

	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}

	// Request more data.
	return 0, nil, nil
}

// calculateTemplateDigest calculate SHA256 digest of a rendered template.
func calculateTemplateDigest(src string, params map[string]string) (string, error) {
	digest := sha256.New()

	srcFile, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("error opening template file %s: %w", src, err)
	}

	defer func() { _ = srcFile.Close() }()

	if err = renderTemplate(srcFile, params, digest); err != nil {
		return "", fmt.Errorf("digest calculation of the template file %s failed: %w", src, err)
	}

	hexDigest := hex.EncodeToString(digest.Sum(nil))

	return hexDigest, nil
}

// createFile under provided path and with provided uid and gid.
//
// The agent typically runs as root, so all path resolution here must refuse
// to follow attacker-controlled symlinks (CWE-59). makeDirectories rejects
// symlinked intermediate components; O_NOFOLLOW rejects a symlinked final
// component so we never open, truncate, or chown an attacker-chosen target.
func createFile(path string, fileCreateData *fileCreateData, permission os.FileMode, truncate bool) (*os.File, error) {
	var err error
	if err = makeDirectories(path, fileManagerDefaultDirectoryPermission, fileCreateData.uid, fileCreateData.gid); err != nil {
		return nil, err
	}

	openFlags := os.O_RDWR | os.O_CREATE | syscall.O_NOFOLLOW

	if truncate {
		openFlags = openFlags | os.O_TRUNC
	}

	var file *os.File
	if file, err = os.OpenFile(path, openFlags, permission); err != nil {
		return nil, fmt.Errorf("error creating file %s: %w", path, err)
	}

	if err = file.Chown(fileCreateData.uid, fileCreateData.gid); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("error setting owner on %s: %w", path, err)
	}

	return file, nil
}

// isFileReady returns true if provided file exists and has expected contents.
func isFileReady(path string, fileMetadata *FileMetadata) (bool, error) {
	fd, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		return false, fmt.Errorf("checking if file is ready failed: %w", err)
	}

	defer func() { _ = fd.Close() }()

	return isFileReadyFd(fd, fileMetadata)
}

// isFileReadyFd returns true if the contents read from fd match fileMetadata's expected digest.
func isFileReadyFd(fd *os.File, fileMetadata *FileMetadata) (bool, error) {
	var expectedHexDigest string
	var digest hash.Hash

	if fileMetadata.SHA256() != "" {
		expectedHexDigest = fileMetadata.SHA256()
		digest = sha256.New()
	} else {
		expectedHexDigest = fileMetadata.MD5
		digest = md5.New()
	}

	if _, err := io.Copy(digest, fd); err != nil {
		return false, fmt.Errorf("calculating local file checksum failed: %w", err)
	}

	calculatedHexDigest := hex.EncodeToString(digest.Sum(nil))

	fileIsReady := calculatedHexDigest == expectedHexDigest

	return fileIsReady, nil
}

const freeDiskOverhead = 1024 * 1024 * 1 // 1MB

type fileCreateData struct {
	path       string
	uid        int
	gid        int
	bytesAvail int64
}

// determineFileCreateData detects uid and gid for the path.
func determineFileCreateData(dst string) (*fileCreateData, error) {

	fileInfo, err := os.Stat(dst)
	if err != nil {
		// if path doesn't exist, try to determine owner of the parent directory
		if errors.Is(err, fs.ErrNotExist) {
			parentDirPath := filepath.Dir(dst)

			if parentDirPath == dst {
				// this should never happen, but in case it does, use the process uid/gid
				return &fileCreateData{
					path:       dst,
					uid:        os.Geteuid(),
					gid:        os.Getgid(),
					bytesAvail: 0,
				}, nil
			}

			return determineFileCreateData(parentDirPath)
		}

		return nil, fmt.Errorf("cannot check file create data: %s - %w", dst, err)
	}

	// if file exists, use its uid/gid
	fileStat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("cannot check file ownership: %s - unsupported OS", dst)
	}

	uid, gid := int(fileStat.Uid), int(fileStat.Gid)

	// get diskspace available
	stat := syscall.Statfs_t{}
	if err = syscall.Statfs(dst, &stat); err != nil {
		return nil, fmt.Errorf("cannot check disk space: %s - %w", dst, err)
	}

	bytesAvail := int64(stat.Bavail) * int64(stat.Bsize)

	return &fileCreateData{
		uid:        uid,
		gid:        gid,
		bytesAvail: bytesAvail,
		path:       dst,
	}, nil
}

// refreshBytesAvail updates the bytesAvail field of the fileCreateData by re-checking the available disk space.
func (fcd *fileCreateData) refreshBytesAvail() error {
	stat := syscall.Statfs_t{}
	if err := syscall.Statfs(fcd.path, &stat); err != nil {
		return fmt.Errorf("cannot check disk space: %s - %w", fcd.path, err)
	}

	fcd.bytesAvail = int64(stat.Bavail) * int64(stat.Bsize)
	return nil
}

// makeDirectories checks if all directories for the dst file exist, if not, create them with provided owner and group.
//
// All ancestor components are inspected with os.Lstat so that a symlinked
// directory (e.g. an attacker's ~/.ssh pointing at /root/.ssh) is rejected
// rather than transparently traversed by the kernel (CWE-59).
func makeDirectories(dst string, permissions os.FileMode, uid, gid int) error {
	if dst == "/" {
		return nil
	}

	dirPath := filepath.Dir(dst)

	// Always validate / materialize ancestors first so that symlinks higher
	// up the path are caught even when dirPath itself already exists.
	if err := makeDirectories(dirPath, permissions, uid, gid); err != nil {
		return err
	}

	dirInfo, err := os.Lstat(dirPath)
	if errors.Is(err, os.ErrNotExist) {
		if err = os.Mkdir(dirPath, permissions); err != nil {
			return fmt.Errorf("cannot create directorty %s: %w", dirPath, err)
		}

		if err = os.Chown(dirPath, uid, gid); err != nil {
			return fmt.Errorf("cannot change owner of %s: %w", dirPath, err)
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dirPath, err)
	}

	if dirInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to traverse symlinked path component %s", dirPath)
	}

	if !dirInfo.IsDir() {
		return fmt.Errorf("cannot create directory, %s is a file", dirPath)
	}

	return nil
}

const (
	templateHostTag = "$(sys.host)"
)

// resolveSourcePath using system tags.
func resolveSourcePath(path string) (string, error) {
	if !strings.Contains(path, templateHostTag) {
		return path, nil
	}

	hostname, err := utils.Hostname()
	if err != nil {
		return "", fmt.Errorf("error getting hostname: %w", err)
	}

	path = strings.ReplaceAll(path, templateHostTag, hostname)

	return path, nil
}

// resolveDestinationPath check if the destination path is a directory and returns the path with the source basename
func resolveDestinationPath(source, destination string) (string, error) {
	if destination == "" {
		return "", fmt.Errorf("destination path is empty")
	}

	fileInfo, err := os.Stat(destination)

	if err != nil {
		// Check if the destination path is a directory
		if destination[len(destination)-1:] == string(os.PathSeparator) {
			return "", fmt.Errorf("destination path %s is a directory", destination)
		}

		if os.IsNotExist(err) {
			// destination doesn't exist, use it as is if it doesn't end with a path separator
			return destination, nil
		}
		return "", err
	}

	if fileInfo.IsDir() {
		// is a directory
		baseName := filepath.Base(source)
		return filepath.Join(destination, baseName), nil
	}
	return destination, nil
}

const partialDownloadSuffix = ".part"
const partialDownloadDirectorySuffix = ".qbee-partials"
const partialDownloadDirectoryPermission = 0700

// GetPartialDownloadFilePath returns <dir>/.<sha256(basename)>.qbee-partials/<sha256(digest)>.part for path.
// Hashing keeps names within NAME_MAX and free of path separators from untrusted input.
func GetPartialDownloadFilePath(path, digest string) string {
	directory := fmt.Sprintf(".%x%s", sha256.Sum256([]byte(filepath.Base(path))), partialDownloadDirectorySuffix)
	name := fmt.Sprintf("%x%s", sha256.Sum256([]byte(digest)), partialDownloadSuffix)

	return filepath.Join(filepath.Dir(path), directory, name)
}

// openPrivatePartialDownloadDirectory creates (if needed) and opens the 0700, agent-owned
// directory dirName within parent.
func openPrivatePartialDownloadDirectory(parent *os.Root, dirName string) (*os.Root, error) {
	if err := parent.Mkdir(dirName, partialDownloadDirectoryPermission); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating partial download directory %s: %w", dirName, err)
	}

	dir, err := openSubdirectoryAnchored(parent, dirName)
	if err != nil {
		return nil, fmt.Errorf("error opening partial download directory %s: %w", dirName, err)
	}

	info, err := dir.Stat(".")
	if err != nil {
		_ = dir.Close()
		return nil, fmt.Errorf("error checking partial download directory %s: %w", dirName, err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		_ = dir.Close()
		return nil, fmt.Errorf("partial download directory %s is not owned by the agent", dirName)
	}

	if err = dir.Chmod(".", partialDownloadDirectoryPermission); err != nil {
		_ = dir.Close()
		return nil, fmt.Errorf("error securing partial download directory %s: %w", dirName, err)
	}

	return dir, nil
}

// openDirectoryAnchored opens dirPath as an os.Root, resolving it relative to the filesystem
// root one component at a time. Symlinked path components are rejected rather than traversed.
func openDirectoryAnchored(dirPath string) (*os.Root, error) {
	dirPath = filepath.Clean(dirPath)

	start := "."
	if filepath.IsAbs(dirPath) {
		start = string(filepath.Separator)
	}

	root, err := os.OpenRoot(start)
	if err != nil {
		return nil, err
	}

	rest := strings.TrimPrefix(dirPath, string(filepath.Separator))

	for component := range strings.SplitSeq(rest, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}

		next, openErr := openSubdirectoryAnchored(root, component)
		_ = root.Close()
		if openErr != nil {
			return nil, openErr
		}

		root = next
	}

	return root, nil
}

// openSubdirectoryAnchored opens the directory name within root, refusing to traverse a symlink.
// The opened directory is compared with the inspected entry, so it cannot be swapped in between
// the check and the open (CWE-59, CWE-367).
func openSubdirectoryAnchored(root *os.Root, name string) (*os.Root, error) {
	linkInfo, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}

	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to traverse symlinked path component %s", name)
	}

	if !linkInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", name)
	}

	subRoot, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}

	openedInfo, err := subRoot.Stat(".")
	if err != nil {
		_ = subRoot.Close()
		return nil, err
	}

	if !os.SameFile(linkInfo, openedInfo) {
		_ = subRoot.Close()
		return nil, fmt.Errorf("path component %s was replaced while opening it", name)
	}

	return subRoot, nil
}

// removeStalePartialDownloads removes partial downloads within dir left over from a previous
// attempt made for a different digest, keeping only keepName.
func removeStalePartialDownloads(dir *os.Root, keepName string) error {
	entries, err := fs.ReadDir(dir.FS(), ".")
	if err != nil {
		return fmt.Errorf("error listing partial download directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == keepName || entry.IsDir() || !strings.HasSuffix(name, partialDownloadSuffix) {
			continue
		}

		// remove the stale partial download
		if err = dir.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("error removing stale partial download %s: %w", name, err)
		}
	}

	return nil
}
