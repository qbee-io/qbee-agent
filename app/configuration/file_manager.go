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

type FileMetadata struct {
	MD5          string            `json:"md5"`
	LastModified int64             `json:"last_modified"`
	Tags         map[string]string `json:"tags,omitempty"`
}

const fileDigestSHA256Tag = "qbee_digest_sha256"

// SHA256 returns hex-encoded sha256 digest of the file (if present), otherwise an empty string.
func (md *FileMetadata) SHA256() string {
	if md.Tags == nil {
		return ""
	}

	return md.Tags[fileDigestSHA256Tag]
}

// downloadFile and return true when file was created. In case the right file already existed, return false.
func (srv *Service) downloadFile(ctx context.Context, src, dst string) (bool, error) {
	var err error

	defer func() {
		if err != nil {
			ReportError(ctx, err, "Unable to download file %s to %s", src, dst)
		}
	}()

	if !strings.HasPrefix(dst, "/") {
		err = fmt.Errorf("absolute file path required, got %s", dst)
		return false, err
	}

	var fileMetadata *FileMetadata
	if fileMetadata, err = srv.getFileMetadata(ctx, src); err != nil {
		return false, err
	}

	var fileReady bool
	if fileReady, err = isFileReady(dst, fileMetadata.SHA256(), fileMetadata.MD5); err != nil || fileReady {
		return false, err
	}

	var srcFile io.ReadCloser
	if srcFile, err = srv.getFile(ctx, src); err != nil {
		return false, err
	}

	defer srcFile.Close()

	var dstFile *os.File
	if dstFile, err = createFile(dst, fileManagerDefaultFilePermission); err != nil {
		return false, err
	}

	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return false, fmt.Errorf("error writing file %s: %w", dst, err)
	}

	ReportInfo(ctx, nil, "Successfully downloaded file %s to %s", src, dst)

	return true, nil
}

// downloadTemplateFile and execute - returns true if file template was executed and resulted in a new dst file.
func (srv *Service) downloadTemplateFile(ctx context.Context, src, dst string, params map[string]string) (bool, error) {
	var err error

	defer func() {
		if err != nil {
			ReportError(ctx, err, "Unable to render template file %s to %s.", src, dst)
		}
	}()

	cacheSrc := filepath.Join(srv.cacheDirectory, FileDistributionCacheDirectory, src)

	if _, err = srv.downloadFile(ctx, src, cacheSrc); err != nil {
		return false, err
	}

	var sha256digest string
	if sha256digest, err = calculateTemplateDigest(cacheSrc, params); err != nil {
		return false, err
	}

	var fileReady bool
	if fileReady, err = isFileReady(dst, sha256digest, ""); err != nil || fileReady {
		return false, err
	}

	var srcFile io.ReadCloser
	if srcFile, err = os.Open(cacheSrc); err != nil {
		return false, fmt.Errorf("error opening template file %s: %w", cacheSrc, err)
	}

	defer srcFile.Close()

	var dstFile io.WriteCloser
	if dstFile, err = createFile(dst, fileManagerDefaultFilePermission); err != nil {
		return false, err
	}

	defer dstFile.Close()

	if err = renderTemplate(srcFile, params, dstFile); err != nil {
		return false, err
	}

	ReportInfo(ctx, nil, "Successfully rendered template file %s to %s", src, dst)

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

		renderedLine, err := renderTemplateLine(line, params, lineNo)
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
func renderTemplateLine(line []byte, params map[string]string, lineNo int) ([]byte, error) {
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

	defer srcFile.Close()

	if err = renderTemplate(srcFile, params, digest); err != nil {
		return "", fmt.Errorf("digest calculation of the template file %s failed: %w", src, err)
	}

	hexDigest := hex.EncodeToString(digest.Sum(nil))

	return hexDigest, nil
}

// createFile under provided path and with provided uid and gid.
func createFile(path string, permission os.FileMode) (*os.File, error) {
	uid, gid, err := determineFileOwner(path)
	if err != nil {
		return nil, err
	}

	if err = makeDirectories(path, fileManagerDefaultDirectoryPermission, uid, gid); err != nil {
		return nil, err
	}

	var file *os.File
	if file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, permission); err != nil {
		return nil, fmt.Errorf("error creating file %s: %w", path, err)
	}

	if err = file.Chown(uid, gid); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("error setting owner on %s: %w", path, err)
	}

	return file, nil
}

// isFileReady returns true if provided file exists and has expected contents.
func isFileReady(path, sha256Digest, md5Digest string) (bool, error) {
	fd, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		return false, fmt.Errorf("checking if file is ready failed: %w", err)
	}

	defer fd.Close()

	var expectedHexDigest string
	var digest hash.Hash

	if sha256Digest != "" {
		expectedHexDigest = sha256Digest
		digest = sha256.New()
	} else {
		expectedHexDigest = md5Digest
		digest = md5.New()
	}

	if _, err = io.Copy(digest, fd); err != nil {
		return false, fmt.Errorf("calculating local file checksum failed: %w", err)
	}

	calculatedHexDigest := hex.EncodeToString(digest.Sum(nil))

	fileIsReady := calculatedHexDigest == expectedHexDigest

	return fileIsReady, nil
}

// determineFileOwner detects uid and gid for the path.
func determineFileOwner(dst string) (int, int, error) {
	fileInfo, err := os.Stat(dst)
	if err != nil {
		// if path doesn't exist, try to determine owner of the parent directory
		if errors.Is(err, fs.ErrNotExist) {
			parentDirPath := filepath.Dir(dst)

			if parentDirPath == dst {
				// this should never happen, but in case it does, use the process uid/gid
				return os.Geteuid(), os.Getgid(), nil
			}

			return determineFileOwner(parentDirPath)
		}

		return 0, 0, fmt.Errorf("cannot check file ownership: %s - %w", dst, err)
	}

	// if file exists, use its uid/gid
	fileStat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("cannot check file ownership: %s - unsupported OS", dst)
	}

	uid, gid := int(fileStat.Uid), int(fileStat.Gid)

	return uid, gid, nil
}

// makeDirectories checks if all directories for the dst file exist, if not, create them with provided owner and group.
func makeDirectories(dst string, permissions os.FileMode, uid, gid int) error {
	if dst == "/" {
		return nil
	}

	dirPath := filepath.Dir(dst)

	dirInfo, err := os.Stat(dirPath)
	if errors.Is(err, os.ErrNotExist) {
		// ensure parent exists
		if err = makeDirectories(dirPath, permissions, uid, gid); err != nil {
			return err
		}

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

	hostname, err := os.Hostname()
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
		// Check of the destination path is a directory
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
